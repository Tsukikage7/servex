package kafka

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/IBM/sarama"

	"github.com/Tsukikage7/servex/messaging/pubsub"
)

// Subscriber 通过 Kafka Consumer Group 订阅消息.
type Subscriber struct {
	client  sarama.Client
	groupID string
	group   sarama.ConsumerGroup
	closed  atomic.Bool
	mu      sync.Mutex
	cancels []context.CancelFunc // 所有订阅的 cancel 函数
	wg      sync.WaitGroup
	opts    subscriberOptions
}

// NewSubscriber 基于已有的 sarama.Client 创建 Subscriber.
func NewSubscriber(client sarama.Client, groupID string, opts ...SubscriberOption) (*Subscriber, error) {
	if client == nil {
		return nil, errors.New("pubsub/kafka: client 不能为空")
	}
	if groupID == "" {
		return nil, errors.New("pubsub/kafka: groupID 不能为空")
	}

	var o subscriberOptions
	for _, opt := range opts {
		opt(&o)
	}

	group, err := sarama.NewConsumerGroupFromClient(groupID, client)
	if err != nil {
		return nil, errors.Join(errors.New("pubsub/kafka: 创建 consumer group 失败"), err)
	}

	return &Subscriber{
		client:  client,
		groupID: groupID,
		group:   group,
		opts:    o,
	}, nil
}

// Subscribe 订阅指定 topic，返回消息 channel.
// channel 在 Subscriber 关闭或 ctx 取消时关闭.
func (s *Subscriber) Subscribe(ctx context.Context, topic string) (<-chan *pubsub.Message, error) {
	if s.closed.Load() {
		return nil, pubsub.ErrClosed
	}
	if topic == "" {
		return nil, pubsub.ErrEmptyTopic
	}

	ch := make(chan *pubsub.Message)
	subCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancels = append(s.cancels, cancel)
	s.mu.Unlock()

	handler := &consumerGroupHandler{ch: ch}

	s.wg.Go(func() {
		defer close(ch)
		for {
			if err := s.group.Consume(subCtx, []string{topic}, handler); err != nil {
				if s.closed.Load() {
					return
				}
				continue
			}
			if subCtx.Err() != nil {
				return
			}
		}
	})

	return ch, nil
}

// Ack 确认消息已处理.
func (s *Subscriber) Ack(ctx context.Context, msg *pubsub.Message) error {
	if msg == nil {
		return pubsub.ErrNilMessage
	}
	session, ok := msg.Metadata["session"].(sarama.ConsumerGroupSession)
	if !ok {
		return pubsub.ErrAckFailed
	}
	cm, ok := msg.Metadata["consumer_message"].(*sarama.ConsumerMessage)
	if !ok {
		return pubsub.ErrAckFailed
	}
	session.MarkMessage(cm, "")
	return nil
}

// Nack 拒绝消息（Kafka 不支持原生 Nack，此处为空操作）.
func (s *Subscriber) Nack(ctx context.Context, msg *pubsub.Message) error {
	if msg == nil {
		return pubsub.ErrNilMessage
	}
	return nil
}

// Close 关闭 Subscriber.
func (s *Subscriber) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	s.mu.Lock()
	for _, cancel := range s.cancels {
		cancel()
	}
	s.cancels = nil
	s.mu.Unlock()
	s.wg.Wait()
	return s.group.Close()
}

// consumerGroupHandler 实现 sarama.ConsumerGroupHandler.
type consumerGroupHandler struct {
	ch chan<- *pubsub.Message
}

func (h *consumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			headers := make(map[string]string, len(msg.Headers))
			for _, rh := range msg.Headers {
				headers[string(rh.Key)] = string(rh.Value)
			}
			pm := &pubsub.Message{
				Topic:   msg.Topic,
				Key:     msg.Key,
				Body:    msg.Value,
				Headers: headers,
				Metadata: map[string]any{
					"partition":        msg.Partition,
					"offset":           msg.Offset,
					"session":          session,
					"consumer_message": msg,
				},
			}
			select {
			case h.ch <- pm:
			case <-session.Context().Done():
				return nil
			}
		case <-session.Context().Done():
			return nil
		}
	}
}
