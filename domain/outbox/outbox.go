// Package outbox 实现事务发件箱模式.
// 事务发件箱模式将消息与业务数据在同一数据库事务中持久化，
// 由异步 Relay 轮询投递到消息队列，保证最终一致性.
// 状态流转:
//
//	Pending(0) ──FetchPending──► Processing(1) ──发送成功──► Sent(2)
//	                                  │
//	                                  └──发送失败──► Failed(3)
//	                                                    │
//	                                          ResetStale ──► Pending(0)
//
// 使用示例:
//
//	store := outboxgorm.NewStore(db)  // import outboxgorm "github.com/Tsukikage7/servex/v2/domain/outbox/gorm"
//	store.AutoMigrate()
//	relay, _ := outbox.NewRelay(store, producer, outbox.WithLogger(log))
//	relay.Start(ctx)
//	defer relay.Stop(ctx)
//	// 业务事务中写入 outbox 消息
//	store.WithTx(ctx, func(txCtx context.Context) error {
//	    // 业务操作...
//	    return store.Save(txCtx, outbox.NewOutboxMessage(&pubsub.Message{
//	        Topic: "order.created",
//	        Key:   []byte("order-123"),
//	        Body:  orderJSON,
//	    }))
//	})
package outbox

import (
	"encoding/json"
	"time"

	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
)

// MessageStatus 消息状态.
type MessageStatus int8

const (
	// StatusPending 待发送.
	StatusPending MessageStatus = 0
	// StatusProcessing 发送中.
	StatusProcessing MessageStatus = 1
	// StatusSent 已发送.
	StatusSent MessageStatus = 2
	// StatusFailed 发送失败.
	StatusFailed MessageStatus = 3
)

// String 返回状态的可读名称.
func (s MessageStatus) String() string {
	switch s {
	case StatusPending:
		return "Pending"
	case StatusProcessing:
		return "Processing"
	case StatusSent:
		return "Sent"
	case StatusFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// OutboxMessage 发件箱消息 GORM 模型.
type OutboxMessage struct {
	ID         uint64        `gorm:"primaryKey;autoIncrement"`
	Topic      string        `gorm:"type:varchar(255);not null"`
	Key        []byte        `gorm:"type:varbinary(255)"`
	Value      []byte        `gorm:"type:mediumblob;not null"`
	Headers    string        `gorm:"type:text"`
	Status     MessageStatus `gorm:"type:tinyint;not null;default:0;index:idx_outbox_status"`
	RetryCount int           `gorm:"type:int;not null;default:0"`
	LastError  string        `gorm:"type:text"`
	CreatedAt  time.Time     `gorm:"autoCreateTime;not null"`
	UpdatedAt  time.Time     `gorm:"autoUpdateTime;not null"`
	SentAt     *time.Time
}

// TableName 指定表名.
func (OutboxMessage) TableName() string {
	return "outbox_messages"
}

// ToMessage 将 OutboxMessage 转换为 pubsub.Message.
func (m *OutboxMessage) ToMessage() *pubsub.Message {
	msg := &pubsub.Message{
		Topic: m.Topic,
		Key:   m.Key,
		Body:  m.Value,
	}
	if m.Headers != "" {
		_ = json.Unmarshal([]byte(m.Headers), &msg.Headers) // Headers 反序列化失败时发送无 header 的消息
	}
	return msg
}

// NewOutboxMessage 从 pubsub.Message 创建 OutboxMessage.
func NewOutboxMessage(msg *pubsub.Message) *OutboxMessage {
	om := &OutboxMessage{
		Topic: msg.Topic,
		Key:   msg.Key,
		Value: msg.Body,
	}
	if len(msg.Headers) > 0 {
		om.Headers = HeadersToJSON(msg.Headers)
	}
	return om
}

// HeadersToJSON 将 headers map 序列化为 JSON 字符串.
func HeadersToJSON(headers map[string]string) string {
	if len(headers) == 0 {
		return ""
	}
	b, err := json.Marshal(headers)
	if err != nil {
		return ""
	}
	return string(b)
}
