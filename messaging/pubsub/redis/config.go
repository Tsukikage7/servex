package redis

import (
	goredis "github.com/redis/go-redis/v9"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// NewPublisherFromConfig 根据连接参数创建 Publisher，内部自动创建 redis.Client.
func NewPublisherFromConfig(addr, password string, db int, log logger.Logger) (*Publisher, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return NewPublisher(client, WithPublisherLogger(log))
}

// NewSubscriberFromConfig 根据连接参数创建 Subscriber，内部自动创建 redis.Client.
// group 和 consumer 用于 Redis Streams 消费者组模式；留空则使用简单 XREAD 模式.
func NewSubscriberFromConfig(addr, password string, db int, group, consumer string, log logger.Logger) (*Subscriber, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	opts := []SubscriberOption{WithSubscriberLogger(log)}
	if group != "" {
		opts = append(opts, WithConsumerGroup(group, consumer))
	}
	return NewSubscriber(client, opts...)
}
