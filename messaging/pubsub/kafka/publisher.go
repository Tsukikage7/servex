// Package kafka 提供基于 Apache Kafka 的 pubsub.Publisher 和 pubsub.Subscriber 实现.
package kafka

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/IBM/sarama"

	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
)

// Publisher 通过 Kafka 发布消息.
type Publisher struct {
	producer sarama.SyncProducer
	closed   atomic.Bool
	mu       sync.Mutex
	opts     publisherOptions
}

// NewPublisher 基于已有的 sarama.Client 创建 Publisher.
// 注意：调用方负责关闭传入的 sarama.Client，Publisher 仅关闭其创建的 producer.
func NewPublisher(client sarama.Client, opts ...PublisherOption) (*Publisher, error) {
	if client == nil {
		return nil, errors.New("pubsub/kafka: client 不能为空")
	}

	var o publisherOptions
	for _, opt := range opts {
		opt(&o)
	}

	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		return nil, errors.Join(errors.New("pubsub/kafka: 创建 producer 失败"), err)
	}

	return &Publisher{producer: producer, opts: o}, nil
}

// Publish 发布一条或多条消息到指定 topic.
func (p *Publisher) Publish(ctx context.Context, topic string, msgs ...*pubsub.Message) error {
	if p.closed.Load() {
		return pubsub.ErrClosed
	}
	if topic == "" {
		return pubsub.ErrEmptyTopic
	}
	if len(msgs) == 0 {
		return pubsub.ErrNoMessages
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	saramaMsgs := make([]*sarama.ProducerMessage, 0, len(msgs))
	for _, msg := range msgs {
		if msg == nil {
			return pubsub.ErrNilMessage
		}
		pm := &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.ByteEncoder(msg.Body),
		}
		if len(msg.Key) > 0 {
			pm.Key = sarama.ByteEncoder(msg.Key)
		}
		for k, v := range msg.Headers {
			pm.Headers = append(pm.Headers, sarama.RecordHeader{
				Key:   []byte(k),
				Value: []byte(v),
			})
		}
		saramaMsgs = append(saramaMsgs, pm)
	}

	if len(saramaMsgs) == 1 {
		partition, offset, err := p.producer.SendMessage(saramaMsgs[0])
		if err != nil {
			return err
		}
		msgs[0].Metadata = map[string]any{
			"partition": partition,
			"offset":    offset,
		}
		return nil
	}

	return p.producer.SendMessages(saramaMsgs)
}

// Close 关闭 producer. 幂等，多次调用不会报错.
func (p *Publisher) Close() error {
	if p.closed.Swap(true) {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.producer.Close()
}
