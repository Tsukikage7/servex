package websocket

import (
	"sync"
	"time"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// LoggingMiddleware 日志中间件.
func LoggingMiddleware(log logger.Logger) Middleware {
	return func(next Handler) Handler {
		return func(client Client, msg *Message) {
			start := time.Now()
			log.Debug("[WebSocket] 收到消息",
				"client_id", client.ID(),
				"type", msg.Type,
				"size", len(msg.Data),
			)

			next(client, msg)

			log.Debug("[WebSocket] 消息处理完成",
				"client_id", client.ID(),
				"duration", time.Since(start),
			)
		}
	}
}

// RecoveryMiddleware Panic 恢复中间件.
func RecoveryMiddleware(log logger.Logger) Middleware {
	return func(next Handler) Handler {
		return func(client Client, msg *Message) {
			defer func() {
				if r := recover(); r != nil {
					log.Error("[WebSocket] 处理器发生 panic",
						"client_id", client.ID(),
						"panic", r,
					)
				}
			}()
			next(client, msg)
		}
	}
}

// RateLimitMiddleware 限流中间件.
// 使用 sync.Map 保证并发安全，并在窗口重置时清理断开连接的客户端.
func RateLimitMiddleware(maxMessages int, window time.Duration) Middleware {
	type clientLimit struct {
		mu        sync.Mutex
		count     int
		resetTime time.Time
	}
	var limits sync.Map // map[string]*clientLimit

	return func(next Handler) Handler {
		return func(client Client, msg *Message) {
			now := time.Now()
			id := client.ID()

			val, _ := limits.LoadOrStore(id, &clientLimit{resetTime: now.Add(window)})
			limit := val.(*clientLimit)

			limit.mu.Lock()
			if now.After(limit.resetTime) {
				limit.count = 0
				limit.resetTime = now.Add(window)

				// 清理断开连接的其他客户端条目，避免内存泄漏
				limits.Range(func(key, value any) bool {
					k := key.(string)
					if k == id {
						return true
					}
					cl := value.(*clientLimit)
					cl.mu.Lock()
					if now.After(cl.resetTime) {
						cl.mu.Unlock()
						limits.Delete(k)
					} else {
						cl.mu.Unlock()
					}
					return true
				})
			}

			if limit.count >= maxMessages {
				limit.mu.Unlock()
				// 超过限制，丢弃消息
				return
			}

			limit.count++
			limit.mu.Unlock()
			next(client, msg)
		}
	}
}

// MessageSizeMiddleware 消息大小限制中间件.
func MessageSizeMiddleware(maxSize int64) Middleware {
	return func(next Handler) Handler {
		return func(client Client, msg *Message) {
			if int64(len(msg.Data)) > maxSize {
				// 消息过大，丢弃
				return
			}
			next(client, msg)
		}
	}
}

// AuthMiddleware 认证中间件（示例）.
// 实际使用时应根据业务需求实现认证逻辑.
func AuthMiddleware(validateToken func(token string) bool) Middleware {
	return func(next Handler) Handler {
		return func(client Client, msg *Message) {
			// 检查客户端是否已认证
			if _, ok := client.Metadata()["authenticated"]; ok {
				next(client, msg)
				return
			}

			// 首条消息应为认证消息
			if msg.Type == TextMessage {
				token := string(msg.Data)
				if validateToken(token) {
					client.SetMetadata("authenticated", true)
					// 发送认证成功响应
					_ = client.Send(&Message{
						Type: TextMessage,
						Data: []byte(`{"type":"auth","status":"ok"}`),
					})
					return
				}
			}

			// 认证失败
			_ = client.Send(&Message{
				Type: TextMessage,
				Data: []byte(`{"type":"auth","status":"error","message":"unauthorized"}`),
			})
			_ = client.Close()
		}
	}
}
