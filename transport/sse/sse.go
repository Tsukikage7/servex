// Package sse 提供 Server-Sent Events (SSE) 服务端支持.
//
// 特性:
//   - 标准 SSE 协议实现
//   - 支持事件类型、ID、重试间隔
//   - 支持客户端管理和广播
//   - 支持中间件扩展
//
// 示例:
//
//	// 创建 SSE 服务器
//	server := sse.NewServer()
//	go server.Run(ctx)
//
//	// HTTP 处理
//	http.HandleFunc("/events", server.ServeHTTP)
//
//	// 发送事件
//	server.Broadcast(&sse.Event{
//	    Event: "message",
//	    Data:  []byte("Hello, World!"),
//	})
package sse

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrClientNotFound 客户端不存在.
	ErrClientNotFound = errors.New("sse: client not found")
	// ErrServerClosed 服务器已关闭.
	ErrServerClosed = errors.New("sse: server is closed")
	// ErrConnectionClosed 连接已关闭.
	ErrConnectionClosed = errors.New("sse: connection closed")
	// ErrNotFlusher ResponseWriter 不支持 Flush.
	ErrNotFlusher = errors.New("sse: response writer does not support flushing")
)

// Event SSE 事件.
type Event struct {
	// ID 事件 ID
	ID string
	// Event 事件类型
	Event string
	// Data 事件数据
	Data []byte
	// Retry 重试间隔（毫秒）
	Retry int
}

// Bytes 将事件序列化为 SSE 格式.
func (e *Event) Bytes() []byte {
	var buf bytes.Buffer

	if e.ID != "" {
		buf.WriteString("id: ")
		buf.WriteString(e.ID)
		buf.WriteByte('\n')
	}
	if e.Event != "" {
		buf.WriteString("event: ")
		buf.WriteString(e.Event)
		buf.WriteByte('\n')
	}
	if e.Retry > 0 {
		buf.WriteString("retry: ")
		buf.WriteString(strconv.Itoa(e.Retry))
		buf.WriteByte('\n')
	}
	if len(e.Data) > 0 {
		buf.WriteString("data: ")
		buf.Write(e.Data)
		buf.WriteByte('\n')
	}
	buf.WriteByte('\n')

	return buf.Bytes()
}

// Config SSE 配置.
type Config struct {
	// BufferSize 客户端缓冲区大小
	BufferSize int `json:"buffer_size" yaml:"buffer_size" mapstructure:"buffer_size"`
	// HeartbeatInterval 心跳间隔
	HeartbeatInterval time.Duration `json:"heartbeat_interval" yaml:"heartbeat_interval" mapstructure:"heartbeat_interval"`
	// RetryInterval 客户端重试间隔（毫秒）
	RetryInterval int `json:"retry_interval" yaml:"retry_interval" mapstructure:"retry_interval"`
	// Headers 自定义响应头
	Headers map[string]string `json:"headers" yaml:"headers" mapstructure:"headers"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		BufferSize:        256,
		HeartbeatInterval: 30 * time.Second,
		RetryInterval:     3000,
		Headers:           make(map[string]string),
	}
}

// Client SSE 客户端.
type Client interface {
	// ID 返回客户端 ID
	ID() string
	// Send 发送事件
	Send(event *Event) error
	// Close 关闭连接
	Close() error
	// Context 返回客户端上下文
	Context() context.Context
	// Metadata 返回元数据
	Metadata() map[string]any
	// SetMetadata 设置元数据
	SetMetadata(key string, value any)
}

// Server SSE 服务器接口.
type Server interface {
	// Run 启动服务器
	Run(ctx context.Context) error
	// ServeHTTP HTTP 处理器
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	// Broadcast 广播事件
	Broadcast(event *Event)
	// BroadcastTo 广播事件给指定客户端
	BroadcastTo(clientIDs []string, event *Event)
	// Send 发送事件给指定客户端
	Send(clientID string, event *Event) error
	// Clients 返回所有客户端
	Clients() []Client
	// Client 返回指定客户端
	Client(id string) (Client, bool)
	// Count 返回客户端数量
	Count() int
	// Close 关闭服务器
	Close() error
	// OnConnect 设置连接回调
	OnConnect(fn func(Client))
	// OnDisconnect 设置断开回调
	OnDisconnect(fn func(Client))
}

// sseClient SSE 客户端实现.
type sseClient struct {
	id       string
	events   chan *Event
	ctx      context.Context
	cancel   context.CancelFunc
	metadata map[string]any
	mu       sync.RWMutex
	closed   bool
}

// newClient 创建客户端.
func newClient(bufferSize int, reqCtx context.Context) *sseClient {
	ctx, cancel := context.WithCancel(reqCtx)
	return &sseClient{
		id:       uuid.New().String(),
		events:   make(chan *Event, bufferSize),
		ctx:      ctx,
		cancel:   cancel,
		metadata: make(map[string]any),
	}
}

func (c *sseClient) ID() string {
	return c.id
}

func (c *sseClient) Send(event *Event) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrConnectionClosed
	}
	c.mu.RUnlock()

	select {
	case c.events <- event:
		return nil
	default:
		// 缓冲区满，丢弃消息
		return nil
	}
}

func (c *sseClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true
	c.cancel()
	close(c.events)
	return nil
}

func (c *sseClient) Context() context.Context {
	return c.ctx
}

func (c *sseClient) Metadata() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]any, len(c.metadata))
	for k, v := range c.metadata {
		result[k] = v
	}
	return result
}

func (c *sseClient) SetMetadata(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = value
}

// server SSE 服务器实现.
type server struct {
	config       *Config
	clients      map[string]*sseClient
	mu           sync.RWMutex
	register     chan *sseClient
	unregister   chan *sseClient
	broadcast    chan *Event
	closed       bool
	done         chan struct{}
	onConnect    func(Client)
	onDisconnect func(Client)
}

// NewServer 创建 SSE 服务器.
func NewServer(config *Config) Server {
	if config == nil {
		config = DefaultConfig()
	}

	return &server{
		config:     config,
		clients:    make(map[string]*sseClient),
		register:   make(chan *sseClient, 256),
		unregister: make(chan *sseClient, 256),
		broadcast:  make(chan *Event, 256),
		done:       make(chan struct{}),
	}
}

func (s *server) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return s.Close()
		case <-s.done:
			return nil
		case client := <-s.register:
			s.mu.Lock()
			s.clients[client.id] = client
			s.mu.Unlock()
			if s.onConnect != nil {
				s.onConnect(client)
			}
		case client := <-s.unregister:
			s.mu.Lock()
			if _, ok := s.clients[client.id]; ok {
				delete(s.clients, client.id)
				_ = client.Close()
			}
			s.mu.Unlock()
			if s.onDisconnect != nil {
				s.onDisconnect(client)
			}
		case event := <-s.broadcast:
			s.mu.RLock()
			for _, client := range s.clients {
				_ = client.Send(event)
			}
			s.mu.RUnlock()
		}
	}
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 检查是否支持 Flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// 设置 SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Nginx 禁用缓冲

	// 设置自定义响应头
	for k, v := range s.config.Headers {
		w.Header().Set(k, v)
	}

	// 创建客户端（从请求上下文派生）
	client := newClient(s.config.BufferSize, r.Context())

	// 注册客户端（带超时保护，防止阻塞）
	select {
	case s.register <- client:
	default:
		// register channel 已满，直接在锁内注册
		s.mu.Lock()
		s.clients[client.id] = client
		s.mu.Unlock()
		if s.onConnect != nil {
			s.onConnect(client)
		}
	}

	// 发送重试间隔
	if s.config.RetryInterval > 0 {
		_, _ = fmt.Fprintf(w, "retry: %d\n\n", s.config.RetryInterval)
		flusher.Flush()
	}

	// 启动心跳
	heartbeat := time.NewTicker(s.config.HeartbeatInterval)
	defer heartbeat.Stop()

	// 事件循环
	for {
		select {
		case <-client.ctx.Done():
			s.unregister <- client
			return
		case event, ok := <-client.events:
			if !ok {
				return
			}
			_, _ = w.Write(event.Bytes())
			flusher.Flush()
		case <-heartbeat.C:
			// 发送心跳注释
			_, _ = fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

func (s *server) Broadcast(event *Event) {
	select {
	case s.broadcast <- event:
	default:
		// 直接发送
		s.mu.RLock()
		for _, client := range s.clients {
			_ = client.Send(event)
		}
		s.mu.RUnlock()
	}
}

func (s *server) BroadcastTo(clientIDs []string, event *Event) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, id := range clientIDs {
		if client, ok := s.clients[id]; ok {
			_ = client.Send(event)
		}
	}
}

func (s *server) Send(clientID string, event *Event) error {
	s.mu.RLock()
	client, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return ErrClientNotFound
	}
	return client.Send(event)
}

func (s *server) Clients() []Client {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clients := make([]Client, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	return clients
}

func (s *server) Client(id string) (Client, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[id]
	return client, ok
}

func (s *server) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

func (s *server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true
	close(s.done)

	for _, client := range s.clients {
		_ = client.Close()
	}
	s.clients = make(map[string]*sseClient)

	return nil
}

func (s *server) OnConnect(fn func(Client)) {
	s.onConnect = fn
}

func (s *server) OnDisconnect(fn func(Client)) {
	s.onDisconnect = fn
}
