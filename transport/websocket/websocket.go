// Package websocket 提供 WebSocket 服务端和客户端支持.
//
// 特性:
//   - 基于 gorilla/websocket 实现
//   - 支持连接管理、心跳检测
//   - 支持广播和点对点消息
//   - 支持中间件扩展
//
// 示例:
//
//	// 创建 WebSocket 服务器
//	hub := websocket.NewHub()
//	go hub.Run()
//
//	// HTTP 升级处理
//	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
//	    websocket.ServeWS(hub, w, r)
//	})
package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/Tsukikage7/servex/v2/errors"
)

// MessageType 消息类型.
type MessageType int

const (
	// TextMessage 文本消息.
	TextMessage MessageType = 1
	// BinaryMessage 二进制消息.
	BinaryMessage MessageType = 2
	// CloseMessage 关闭消息.
	CloseMessage MessageType = 8
	// PingMessage Ping 消息.
	PingMessage MessageType = 9
	// PongMessage Pong 消息.
	PongMessage MessageType = 10
)

var (
	// ErrClientNotFound 客户端不存在.
	ErrClientNotFound = errors.New(60605, "transport.websocket.client_not_found", "客户端不存在")
	// ErrHubClosed Hub 已关闭.
	ErrHubClosed = errors.New(60606, "transport.websocket.hub_closed", "Hub 已关闭")
	// ErrConnectionClosed 连接已关闭.
	ErrConnectionClosed = errors.New(60607, "transport.websocket.connection_closed", "连接已关闭")
	// ErrWriteTimeout 写超时.
	ErrWriteTimeout = errors.New(60608, "transport.websocket.write_timeout", "写超时")
	// ErrMessageTooLarge 消息过大.
	ErrMessageTooLarge = errors.New(60609, "transport.websocket.message_too_large", "消息过大")
	// ErrInvalidMessage 无效消息.
	ErrInvalidMessage = errors.New(60610, "transport.websocket.invalid_message", "无效消息")
	// ErrUpgradeFailed 升级失败.
	ErrUpgradeFailed = errors.New(60611, "transport.websocket.upgrade_failed", "升级失败")
)

// Message WebSocket 消息.
type Message struct {
	// Type 消息类型
	Type MessageType
	// Data 消息数据
	Data []byte
	// ClientID 发送者 ID（仅接收时有效）
	ClientID string
	// Timestamp 时间戳
	Timestamp time.Time
}

// Config WebSocket 配置.
type Config struct {
	// ReadBufferSize 读缓冲区大小
	ReadBufferSize int `json:"read_buffer_size" yaml:"read_buffer_size" mapstructure:"read_buffer_size"`
	// WriteBufferSize 写缓冲区大小
	WriteBufferSize int `json:"write_buffer_size" yaml:"write_buffer_size" mapstructure:"write_buffer_size"`
	// MaxMessageSize 最大消息大小
	MaxMessageSize int64 `json:"max_message_size" yaml:"max_message_size" mapstructure:"max_message_size"`
	// WriteTimeout 写超时
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout" mapstructure:"write_timeout"`
	// ReadTimeout 读超时
	ReadTimeout time.Duration `json:"read_timeout" yaml:"read_timeout" mapstructure:"read_timeout"`
	// PingInterval Ping 间隔
	PingInterval time.Duration `json:"ping_interval" yaml:"ping_interval" mapstructure:"ping_interval"`
	// PongTimeout Pong 超时
	PongTimeout time.Duration `json:"pong_timeout" yaml:"pong_timeout" mapstructure:"pong_timeout"`
	// EnableCompression 启用压缩
	EnableCompression bool `json:"enable_compression" yaml:"enable_compression" mapstructure:"enable_compression"`
	// CheckOrigin 跨域检查函数
	CheckOrigin func(origin string) bool `json:"-" yaml:"-" mapstructure:"-"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
		MaxMessageSize:    512 * 1024, // 512KB
		WriteTimeout:      10 * time.Second,
		ReadTimeout:       60 * time.Second,
		PingInterval:      30 * time.Second,
		PongTimeout:       60 * time.Second,
		EnableCompression: true,
		// CheckOrigin 默认 nil，由底层库检查 Origin 匹配 Host
		CheckOrigin: nil,
	}
}

// Client WebSocket 客户端接口.
type Client interface {
	// ID 返回客户端 ID
	ID() string
	// Send 发送消息
	Send(msg *Message) error
	// Close 关闭连接
	Close() error
	// Context 返回客户端上下文
	Context() context.Context
	// SetContext 设置客户端上下文
	SetContext(ctx context.Context)
	// Metadata 返回元数据
	Metadata() map[string]any
	// SetMetadata 设置元数据
	SetMetadata(key string, value any)
}

// Hub 连接管理中心接口.
type Hub interface {
	// Run 启动 Hub
	Run(ctx context.Context) error
	// Register 注册客户端
	Register(client Client)
	// Unregister 注销客户端
	Unregister(client Client)
	// Broadcast 广播消息给所有客户端
	Broadcast(msg *Message)
	// BroadcastTo 广播消息给指定客户端
	BroadcastTo(clientIDs []string, msg *Message)
	// Send 发送消息给指定客户端
	Send(clientID string, msg *Message) error
	// Clients 返回所有客户端
	Clients() []Client
	// Client 返回指定客户端
	Client(id string) (Client, bool)
	// Count 返回客户端数量
	Count() int
	// Close 关闭 Hub
	Close() error
}

// Handler 消息处理器.
type Handler func(client Client, msg *Message)

// Middleware 中间件.
type Middleware func(Handler) Handler

// hub Hub 实现.
type hub struct {
	mu          sync.RWMutex
	clients     map[string]Client
	register    chan Client
	unregister  chan Client
	broadcast   chan *Message
	handler     Handler
	middlewares []Middleware
	closed      bool
	done        chan struct{}
}

// NewHub 创建新的 Hub.
func NewHub(handler Handler, middlewares ...Middleware) Hub {
	h := &hub{
		clients:     make(map[string]Client),
		register:    make(chan Client, 256),
		unregister:  make(chan Client, 256),
		broadcast:   make(chan *Message, 256),
		handler:     handler,
		middlewares: middlewares,
		done:        make(chan struct{}),
	}

	// 应用中间件
	if h.handler != nil {
		for i := len(middlewares) - 1; i >= 0; i-- {
			h.handler = middlewares[i](h.handler)
		}
	}

	return h
}

// Run 启动 Hub.
func (h *hub) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return h.Close()
		case <-h.done:
			return nil
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID()] = client
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID()]; ok {
				delete(h.clients, client.ID())
				_ = client.Close()
			}
			h.mu.Unlock()
		case msg := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				_ = client.Send(msg)
			}
			h.mu.RUnlock()
		}
	}
}

// Register 注册客户端.
func (h *hub) Register(client Client) {
	select {
	case h.register <- client:
	default:
		// 通道满时直接处理
		h.mu.Lock()
		h.clients[client.ID()] = client
		h.mu.Unlock()
	}
}

// Unregister 注销客户端.
func (h *hub) Unregister(client Client) {
	select {
	case h.unregister <- client:
	default:
		h.mu.Lock()
		if _, ok := h.clients[client.ID()]; ok {
			delete(h.clients, client.ID())
			_ = client.Close()
		}
		h.mu.Unlock()
	}
}

// Broadcast 广播消息.
func (h *hub) Broadcast(msg *Message) {
	select {
	case h.broadcast <- msg:
	default:
		// 通道满时直接发送
		h.mu.RLock()
		for _, client := range h.clients {
			_ = client.Send(msg)
		}
		h.mu.RUnlock()
	}
}

// BroadcastTo 广播消息给指定客户端.
func (h *hub) BroadcastTo(clientIDs []string, msg *Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, id := range clientIDs {
		if client, ok := h.clients[id]; ok {
			_ = client.Send(msg)
		}
	}
}

// Send 发送消息给指定客户端.
func (h *hub) Send(clientID string, msg *Message) error {
	h.mu.RLock()
	client, ok := h.clients[clientID]
	h.mu.RUnlock()

	if !ok {
		return ErrClientNotFound
	}
	return client.Send(msg)
}

// Clients 返回所有客户端.
func (h *hub) Clients() []Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := make([]Client, 0, len(h.clients))
	for _, c := range h.clients {
		clients = append(clients, c)
	}
	return clients
}

// Client 返回指定客户端.
func (h *hub) Client(id string) (Client, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	client, ok := h.clients[id]
	return client, ok
}

// Count 返回客户端数量.
func (h *hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Close 关闭 Hub.
func (h *hub) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return nil
	}
	h.closed = true
	close(h.done)

	// 关闭所有客户端
	for _, client := range h.clients {
		_ = client.Close()
	}
	h.clients = make(map[string]Client)

	return nil
}

// HandleMessage 处理客户端消息.
func (h *hub) HandleMessage(client Client, msg *Message) {
	if h.handler != nil {
		h.handler(client, msg)
	}
}
