package websocket

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// gorillaClient gorilla/websocket 客户端实现.
type gorillaClient struct {
	id       string
	conn     *websocket.Conn
	hub      *hub
	send     chan *Message
	ctx      context.Context
	cancel   context.CancelFunc
	metadata map[string]any
	mu       sync.RWMutex
	closed   bool
	config   *Config
}

// newGorillaClient 创建 gorilla 客户端.
// parentCtx 通常为 HTTP 请求上下文，用于关联生命周期.
func newGorillaClient(h *hub, conn *websocket.Conn, config *Config, parentCtx context.Context) *gorillaClient {
	ctx, cancel := context.WithCancel(parentCtx)
	return &gorillaClient{
		id:       uuid.New().String(),
		conn:     conn,
		hub:      h,
		send:     make(chan *Message, 256),
		ctx:      ctx,
		cancel:   cancel,
		metadata: make(map[string]any),
		config:   config,
	}
}

// ID 返回客户端 ID.
func (c *gorillaClient) ID() string {
	return c.id
}

// Send 发送消息.
func (c *gorillaClient) Send(msg *Message) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrConnectionClosed
	}
	c.mu.RUnlock()

	select {
	case c.send <- msg:
		return nil
	case <-time.After(c.config.WriteTimeout):
		return ErrWriteTimeout
	}
}

// Close 关闭连接.
func (c *gorillaClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true
	c.cancel()
	close(c.send)
	return c.conn.Close()
}

// Context 返回上下文.
func (c *gorillaClient) Context() context.Context {
	return c.ctx
}

// SetContext 设置上下文.
func (c *gorillaClient) SetContext(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ctx = ctx
}

// Metadata 返回元数据.
func (c *gorillaClient) Metadata() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]any, len(c.metadata))
	for k, v := range c.metadata {
		result[k] = v
	}
	return result
}

// SetMetadata 设置元数据.
func (c *gorillaClient) SetMetadata(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = value
}

// readPump 读取消息循环.
func (c *gorillaClient) readPump() {
	defer func() {
		c.hub.Unregister(c)
	}()

	c.conn.SetReadLimit(c.config.MaxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(c.config.PongTimeout))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(c.config.PongTimeout))
	})

	for {
		msgType, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// 非正常关闭，可以记录日志
			}
			break
		}

		msg := &Message{
			Type:      MessageType(msgType),
			Data:      data,
			ClientID:  c.id,
			Timestamp: time.Now(),
		}

		c.hub.HandleMessage(c, msg)
	}
}

// writePump 写入消息循环.
func (c *gorillaClient) writePump() {
	ticker := time.NewTicker(c.config.PingInterval)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case msg, ok := <-c.send:
			if !ok {
				// 通道已关闭
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			_ = c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
			if err := c.conn.WriteMessage(int(msg.Type), msg.Data); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Upgrader WebSocket 升级器.
type Upgrader struct {
	config   *Config
	upgrader websocket.Upgrader
}

// NewUpgrader 创建升级器.
func NewUpgrader(config *Config) *Upgrader {
	if config == nil {
		config = DefaultConfig()
	}

	u := &Upgrader{
		config: config,
		upgrader: websocket.Upgrader{
			ReadBufferSize:    config.ReadBufferSize,
			WriteBufferSize:   config.WriteBufferSize,
			EnableCompression: config.EnableCompression,
		},
	}

	if config.CheckOrigin != nil {
		u.upgrader.CheckOrigin = func(r *http.Request) bool {
			return config.CheckOrigin(r.Header.Get("Origin"))
		}
	} else {
		// 未配置 CheckOrigin 时仅在初始化时打印一次警告，不在每���请求时打印
		slog.Warn("[WebSocket] 未配置 CheckOrigin，默认允许所有 Origin，生产环境请务必配置")
		u.upgrader.CheckOrigin = func(r *http.Request) bool {
			return true
		}
	}

	return u
}

// Upgrade 升级 HTTP 连接为 WebSocket.
func (u *Upgrader) Upgrade(h *hub, w http.ResponseWriter, r *http.Request) (Client, error) {
	conn, err := u.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}

	client := newGorillaClient(h, conn, u.config, r.Context())
	h.Register(client)

	// 启动读写协程
	go client.writePump()
	go client.readPump()

	return client, nil
}

// ServeWS HTTP 处理函数，升级连接并处理 WebSocket.
func ServeWS(h Hub, w http.ResponseWriter, r *http.Request, config *Config) error {
	hub, ok := h.(*hub)
	if !ok {
		return ErrUpgradeFailed
	}

	upgrader := NewUpgrader(config)
	_, err := upgrader.Upgrade(hub, w, r)
	return err
}

// HTTPHandler 返回 HTTP 处理器.
func HTTPHandler(h Hub, config *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = ServeWS(h, w, r, config)
	}
}
