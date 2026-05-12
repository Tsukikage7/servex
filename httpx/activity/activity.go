// Package activity 提供用户活跃时间追踪功能.
//
// 设计原则：
//   - 异步解耦：通过消息队列异步记录，不阻塞主业务
//   - 分层存储：Redis 存热数据，数据库存冷数据
//   - 幂等设计：同一时间窗口内多次请求只记录一次
//   - 降级容错：活跃服务不可用时不影响核心业务
//
// 架构：
//
//	请求 -> Gateway -> Kafka -> Consumer -> Redis/DB
//	                      ↓
//	              ClickHouse (分析)
//
// 示例：
//
//	// 1. 初始化 Tracker
//	tracker := activity.NewTracker(
//	    activity.WithRedis(redisClient),
//	    activity.WithProducer(kafkaProducer),
//	)
//
//	// 2. 使用中间件自动追踪
//	handler = activity.HTTPMiddleware(tracker)(handler)
//
//	// 3. 查询用户状态
//	status := tracker.GetStatus(ctx, "user123")
//	fmt.Println(status.IsOnline)      // true
//	fmt.Println(status.LastActiveAt)  // 2024-01-01 12:00:00
package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Event 活跃事件.
type Event struct {
	// UserID 用户 ID
	UserID string `json:"user_id"`

	// Timestamp 事件时间戳
	Timestamp time.Time `json:"timestamp"`

	// EventType 事件类型
	EventType EventType `json:"event_type"`

	// Platform 平台
	Platform string `json:"platform,omitempty"`

	// DeviceID 设备 ID
	DeviceID string `json:"device_id,omitempty"`

	// IP 客户端 IP
	IP string `json:"ip,omitempty"`

	// Path 请求路径
	Path string `json:"path,omitempty"`

	// Extra 扩展数据
	Extra map[string]string `json:"extra,omitzero"`
}

// EventType 事件类型.
type EventType string

const (
	EventTypeRequest   EventType = "request"   // 普通请求
	EventTypeHeartbeat EventType = "heartbeat" // 心跳
	EventTypeLogin     EventType = "login"     // 登录
	EventTypeLogout    EventType = "logout"    // 登出
	EventTypePageView  EventType = "pageview"  // 页面浏览
)

// Status 用户活跃状态.
type Status struct {
	// UserID 用户 ID
	UserID string `json:"user_id"`

	// IsOnline 是否在线
	IsOnline bool `json:"is_online"`

	// LastActiveAt 最后活跃时间
	LastActiveAt time.Time `json:"last_active_at"`

	// LastPlatform 最后使用平台
	LastPlatform string `json:"last_platform,omitempty"`

	// LastIP 最后 IP
	LastIP string `json:"last_ip,omitempty"`

	// OnlineDuration 本次在线时长（秒）
	OnlineDuration int64 `json:"online_duration,omitempty"`
}

// IsActive 检查用户在指定时间内是否活跃.
func (s *Status) IsActive(within time.Duration) bool {
	if s.LastActiveAt.IsZero() {
		return false
	}
	return time.Since(s.LastActiveAt) <= within
}

// Producer 消息生产者接口.
type Producer interface {
	// Publish 发布活跃事件到消息队列
	Publish(ctx context.Context, topic string, event *Event) error
}

// Store 存储接口.
type Store interface {
	// SetLastActive 设置用户最后活跃时间
	SetLastActive(ctx context.Context, userID string, event *Event) error

	// GetLastActive 获取用户最后活跃时间
	GetLastActive(ctx context.Context, userID string) (*Status, error)

	// GetMultiLastActive 批量获取用户最后活跃时间
	GetMultiLastActive(ctx context.Context, userIDs []string) (map[string]*Status, error)

	// SetOnline 设置用户在线状态
	SetOnline(ctx context.Context, userID string, ttl time.Duration) error

	// IsOnline 检查用户是否在线
	IsOnline(ctx context.Context, userID string) (bool, error)

	// GetOnlineCount 获取在线用户数
	GetOnlineCount(ctx context.Context) (int64, error)
}

// UserIDExtractor 用户 ID 提取器.
type UserIDExtractor func(ctx context.Context) string

// Tracker 活跃追踪器.
type Tracker struct {
	opts *options
}

// NewTracker 创建活跃追踪器.
func NewTracker(opts ...Option) *Tracker {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &Tracker{opts: o}
}

// Track 记录活跃事件.
func (t *Tracker) Track(ctx context.Context, event *Event) error {
	if event.UserID == "" {
		return nil // 未登录用户不追踪
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// 检查是否在去重窗口内
	if t.opts.dedupeWindow > 0 && t.isDuplicate(ctx, event) {
		return nil
	}

	// 异步发送到消息队列
	if t.opts.producer != nil && t.opts.asyncMode {
		go func() {
			ctx := context.Background() // 使用新 context 避免请求结束后 ctx 取消
			if err := t.opts.producer.Publish(ctx, t.opts.topic, event); err != nil {
				// 记录错误但不影响主流程
				if t.opts.logger != nil {
					t.opts.logger.Error("[Activity] 发布事件失败", "error", err)
				}
			}
		}()
		return nil
	}

	// 同步写入存储（用于消费端或非异步模式）
	if t.opts.store != nil {
		if err := t.opts.store.SetLastActive(ctx, event.UserID, event); err != nil {
			return fmt.Errorf("activity: failed to set last active: %w", err)
		}
		// 设置在线状态
		if err := t.opts.store.SetOnline(ctx, event.UserID, t.opts.onlineTTL); err != nil {
			return fmt.Errorf("activity: failed to set online: %w", err)
		}
	}

	return nil
}

// GetStatus 获取用户活跃状态.
func (t *Tracker) GetStatus(ctx context.Context, userID string) (*Status, error) {
	if t.opts.store == nil {
		return nil, fmt.Errorf("activity: store not configured")
	}

	status, err := t.opts.store.GetLastActive(ctx, userID)
	if err != nil {
		return nil, err
	}

	if status == nil {
		status = &Status{UserID: userID}
	}

	// 检查是否在线
	online, err := t.opts.store.IsOnline(ctx, userID)
	if err == nil {
		status.IsOnline = online
	}

	return status, nil
}

// GetMultiStatus 批量获取用户活跃状态.
func (t *Tracker) GetMultiStatus(ctx context.Context, userIDs []string) (map[string]*Status, error) {
	if t.opts.store == nil {
		return nil, fmt.Errorf("activity: store not configured")
	}
	return t.opts.store.GetMultiLastActive(ctx, userIDs)
}

// IsOnline 检查用户是否在线.
func (t *Tracker) IsOnline(ctx context.Context, userID string) bool {
	if t.opts.store == nil {
		return false
	}
	online, _ := t.opts.store.IsOnline(ctx, userID)
	return online
}

// GetOnlineCount 获取在线用户数.
func (t *Tracker) GetOnlineCount(ctx context.Context) (int64, error) {
	if t.opts.store == nil {
		return 0, fmt.Errorf("activity: store not configured")
	}
	return t.opts.store.GetOnlineCount(ctx)
}

// isDuplicate 检查是否重复事件.
func (t *Tracker) isDuplicate(ctx context.Context, event *Event) bool {
	if t.opts.store == nil {
		return false
	}

	status, err := t.opts.store.GetLastActive(ctx, event.UserID)
	if err != nil || status == nil {
		return false
	}

	// 如果在去重窗口内，认为是重复
	return time.Since(status.LastActiveAt) < t.opts.dedupeWindow
}

// MarshalEvent 序列化事件.
func MarshalEvent(event *Event) ([]byte, error) {
	return json.Marshal(event)
}

// UnmarshalEvent 反序列化事件.
func UnmarshalEvent(data []byte) (*Event, error) {
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	return &event, nil
}
