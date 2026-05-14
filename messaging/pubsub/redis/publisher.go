// Package redis 提供基于 Redis Streams 的 pubsub.Publisher 和 pubsub.Subscriber 实现.
package redis

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/redis/go-redis/v9"

	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
)

// Publisher 通过 Redis Streams 发布消息.
type Publisher struct {
	client redis.Cmdable
	closed atomic.Bool
	opts   publisherOptions
}

// NewPublisher 基于已有的 redis.Cmdable 创建 Publisher.
func NewPublisher(client redis.Cmdable, opts ...PublisherOption) (*Publisher, error) {
	if client == nil {
		return nil, errors.New("pubsub/redis: client 不能为空")
	}

	o := publisherOptions{
		approx: true,
	}
	for _, opt := range opts {
		opt(&o)
	}

	return &Publisher{client: client, opts: o}, nil
}

// Publish 发布一条或多条消息到指定 stream（topic）.
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

	for _, msg := range msgs {
		if msg == nil {
			return pubsub.ErrNilMessage
		}

		values := map[string]any{
			"body": msg.Body,
		}
		if len(msg.Key) > 0 {
			values["key"] = string(msg.Key)
		}
		for k, v := range msg.Headers {
			values["header:"+k] = v
		}

		args := &redis.XAddArgs{
			Stream: topic,
			ID:     "*",
			Values: values,
		}
		if p.opts.maxLen > 0 {
			args.MaxLen = p.opts.maxLen
			args.Approx = p.opts.approx
		}

		id, err := p.client.XAdd(ctx, args).Result()
		if err != nil {
			return fmt.Errorf("pubsub/redis: XAdd 失败: %w", err)
		}
		if msg.Metadata == nil {
			msg.Metadata = make(map[string]any)
		}
		msg.Metadata["stream_id"] = id
	}

	return nil
}

// Close 关闭 Publisher. 幂等.
// 注意：不关闭底层 Redis 连接（连接由调用方管理）.
func (p *Publisher) Close() error {
	p.closed.Store(true)
	return nil
}
