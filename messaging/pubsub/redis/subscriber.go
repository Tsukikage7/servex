package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
)

// Subscriber 通过 Redis Streams 订阅消息.
// 支持消费者组（XREADGROUP）和简单读取（XREAD）两种模式.
type Subscriber struct {
	client  goredis.Cmdable
	closed  atomic.Bool
	mu      sync.Mutex
	cancels []context.CancelFunc // 所有订阅的 cancel 函数
	wg      sync.WaitGroup
	opts    subscriberOptions
}

// NewSubscriber 基于已有的 redis.Cmdable 创建 Subscriber.
func NewSubscriber(client goredis.Cmdable, opts ...SubscriberOption) (*Subscriber, error) {
	if client == nil {
		return nil, errors.New("pubsub/redis: client 不能为空")
	}

	o := subscriberOptions{
		block: true,
	}
	for _, opt := range opts {
		opt(&o)
	}

	return &Subscriber{client: client, opts: o}, nil
}

// Subscribe 订阅指定 stream（topic），返回消息 channel.
// 若配置了 ConsumerGroup，使用 XREADGROUP；否则使用 XREAD.
func (s *Subscriber) Subscribe(ctx context.Context, topic string) (<-chan *pubsub.Message, error) {
	if s.closed.Load() {
		return nil, pubsub.ErrClosed
	}
	if topic == "" {
		return nil, pubsub.ErrEmptyTopic
	}

	subCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancels = append(s.cancels, cancel)
	s.mu.Unlock()

	msgCh := make(chan *pubsub.Message)

	if s.opts.groupID != "" {
		// 确保消费者组存在（从最新消息开始）
		_ = s.client.XGroupCreateMkStream(subCtx, topic, s.opts.groupID, "$").Err()

		s.wg.Go(func() {
			defer close(msgCh)
			s.readGroup(subCtx, topic, msgCh)
		})
	} else {
		s.wg.Go(func() {
			defer close(msgCh)
			s.readStream(subCtx, topic, msgCh)
		})
	}

	return msgCh, nil
}

// readGroup 通过 XREADGROUP 消费消息.
func (s *Subscriber) readGroup(ctx context.Context, stream string, out chan<- *pubsub.Message) {
	consumer := s.opts.consumer
	if consumer == "" {
		consumer = "default"
	}

	lastID := ">"
	for {
		if ctx.Err() != nil {
			return
		}

		var blockDur time.Duration
		if s.opts.block {
			blockDur = 2 * time.Second
		}

		streams, err := s.client.XReadGroup(ctx, &goredis.XReadGroupArgs{
			Group:    s.opts.groupID,
			Consumer: consumer,
			Streams:  []string{stream, lastID},
			Count:    10,
			Block:    blockDur,
			NoAck:    false,
		}).Result()
		if err != nil {
			if errors.Is(err, goredis.Nil) || ctx.Err() != nil {
				continue
			}
			if s.opts.logger != nil {
				s.opts.logger.Errorf("[PubSub] Redis XREADGROUP 错误 stream=%s group=%s error=%v", stream, s.opts.groupID, err)
			}
			// 退避等待，避免错误风暴
			backoffTimer := time.NewTimer(time.Second)
			select {
			case <-backoffTimer.C:
			case <-ctx.Done():
				backoffTimer.Stop()
				return
			}
			continue
		}

		for _, xstream := range streams {
			for _, xmsg := range xstream.Messages {
				msg := convertXMessage(xstream.Stream, xmsg)
				select {
				case out <- msg:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// readStream 通过 XREAD 消费消息（无消费者组）.
func (s *Subscriber) readStream(ctx context.Context, stream string, out chan<- *pubsub.Message) {
	lastID := "$"
	for {
		if ctx.Err() != nil {
			return
		}

		var blockDur time.Duration
		if s.opts.block {
			blockDur = 2 * time.Second
		}

		streams, err := s.client.XRead(ctx, &goredis.XReadArgs{
			Streams: []string{stream, lastID},
			Count:   10,
			Block:   blockDur,
		}).Result()
		if err != nil {
			if errors.Is(err, goredis.Nil) || ctx.Err() != nil {
				continue
			}
			if s.opts.logger != nil {
				s.opts.logger.Errorf("[PubSub] Redis XREAD 错误 stream=%s error=%v", stream, err)
			}
			// 退避等待，避免错误风暴
			backoffTimer := time.NewTimer(time.Second)
			select {
			case <-backoffTimer.C:
			case <-ctx.Done():
				backoffTimer.Stop()
				return
			}
			continue
		}

		for _, xstream := range streams {
			for _, xmsg := range xstream.Messages {
				if xmsg.ID > lastID || lastID == "$" {
					lastID = xmsg.ID
				}
				msg := convertXMessage(xstream.Stream, xmsg)
				select {
				case out <- msg:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// Ack 确认消息已处理（XACK）. 仅在消费者组模式下有效.
func (s *Subscriber) Ack(ctx context.Context, msg *pubsub.Message) error {
	if msg == nil {
		return pubsub.ErrNilMessage
	}
	streamID, ok := msg.Metadata["stream_id"].(string)
	if !ok {
		return pubsub.ErrAckFailed
	}
	if s.opts.groupID == "" {
		return nil // 非消费者组模式，无需 Ack
	}
	if err := s.client.XAck(ctx, msg.Topic, s.opts.groupID, streamID).Err(); err != nil {
		return fmt.Errorf("%w: %v", pubsub.ErrAckFailed, err)
	}
	return nil
}

// Nack 拒绝消息（Redis Streams 没有原生 Nack，此处为空操作）.
func (s *Subscriber) Nack(_ context.Context, msg *pubsub.Message) error {
	if msg == nil {
		return pubsub.ErrNilMessage
	}
	return nil
}

// Close 关闭 Subscriber. 幂等.
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
	return nil
}

// convertXMessage 将 Redis XMessage 转换为 pubsub.Message.
func convertXMessage(stream string, xmsg goredis.XMessage) *pubsub.Message {
	msg := &pubsub.Message{
		Topic: stream,
		Metadata: map[string]any{
			"stream_id": xmsg.ID,
		},
	}

	headers := make(map[string]string)
	for k, v := range xmsg.Values {
		switch k {
		case "body":
			switch val := v.(type) {
			case []byte:
				msg.Body = val
			case string:
				msg.Body = []byte(val)
			}
		case "key":
			switch val := v.(type) {
			case []byte:
				msg.Key = val
			case string:
				msg.Key = []byte(val)
			}
		default:
			if after, found := strings.CutPrefix(k, "header:"); found {
				if str, ok := v.(string); ok {
					headers[after] = str
				}
			}
		}
	}
	if len(headers) > 0 {
		msg.Headers = headers
	}

	return msg
}
