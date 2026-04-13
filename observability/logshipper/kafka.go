package logshipper

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
)

// KafkaSink Kafka 日志投递，将日志序列化为 JSON 后发布到 Kafka topic.
type KafkaSink struct {
	publisher pubsub.Publisher
	topic     string
}

// KafkaOption KafkaSink 选项.
type KafkaOption func(*KafkaSink)

// WithTopic 设置 Kafka topic，默认 "logs".
func WithTopic(topic string) KafkaOption {
	return func(s *KafkaSink) {
		if topic != "" {
			s.topic = topic
		}
	}
}

// NewKafkaSink 创建 Kafka 日志投递目标.
func NewKafkaSink(publisher pubsub.Publisher, opts ...KafkaOption) *KafkaSink {
	s := &KafkaSink{
		publisher: publisher,
		topic:     "logs",
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Write 将日志条目 JSON 序列化后逐条发布到 Kafka topic.
func (s *KafkaSink) Write(ctx context.Context, entries []Entry) error {
	msgs := make([]*pubsub.Message, 0, len(entries))
	for _, e := range entries {
		body, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("logshipper/kafka: marshal entry: %w", err)
		}
		msgs = append(msgs, &pubsub.Message{
			Body: body,
		})
	}

	if len(msgs) == 0 {
		return nil
	}

	if err := s.publisher.Publish(ctx, s.topic, msgs...); err != nil {
		return fmt.Errorf("logshipper/kafka: publish to topic %s: %w", s.topic, err)
	}
	return nil
}

// Close 不关闭 publisher，其生命周期由调用者管理.
func (s *KafkaSink) Close() error {
	return nil
}
