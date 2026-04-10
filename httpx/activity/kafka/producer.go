// Package kafka 提供基于 Kafka 的活跃事件生产者实现.
package kafka

import (
	"context"
	"encoding/json"

	"github.com/Tsukikage7/servex/httpx/activity"
	"github.com/Tsukikage7/servex/messaging/pubsub"
)

// Producer Kafka 消息生产者.
type Producer struct {
	publisher pubsub.Publisher
}

// NewProducer 创建 Kafka 生产者.
func NewProducer(publisher pubsub.Publisher) *Producer {
	return &Producer{publisher: publisher}
}

// Publish 发布活跃事件到 Kafka.
func (p *Producer) Publish(ctx context.Context, topic string, event *activity.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := &pubsub.Message{
		Topic: topic,
		Key:   []byte(event.UserID), // 使用 user_id 作为分区键，保证同一用户的消息有序
		Body:  data,
		Headers: map[string]string{
			"event_type": string(event.EventType),
			"user_id":    event.UserID,
		},
	}

	return p.publisher.Publish(ctx, topic, msg)
}

// 确保 Producer 实现了 activity.Producer 接口.
var _ activity.Producer = (*Producer)(nil)
