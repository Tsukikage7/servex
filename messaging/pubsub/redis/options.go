package redis

import "github.com/Tsukikage7/servex/v2/observability/logger"

type publisherOptions struct {
	logger logger.Logger
	maxLen int64 // stream max length (0 = unlimited)
	approx bool  // use MAXLEN ~ (approximate trimming)
}

// PublisherOption 配置 Redis Streams Publisher.
type PublisherOption func(*publisherOptions)

// WithPublisherLogger 设置日志器.
func WithPublisherLogger(log logger.Logger) PublisherOption {
	return func(o *publisherOptions) {
		o.logger = logger.WithComponent(log, "PubSub")
	}
}

// WithMaxLen 设置 Stream 最大长度. approx=true 使用近似裁剪（性能更好）.
func WithMaxLen(maxLen int64, approx bool) PublisherOption {
	return func(o *publisherOptions) {
		o.maxLen = maxLen
		o.approx = approx
	}
}

type subscriberOptions struct {
	logger   logger.Logger
	groupID  string
	consumer string
	block    bool
}

// SubscriberOption 配置 Redis Streams Subscriber.
type SubscriberOption func(*subscriberOptions)

// WithSubscriberLogger 设置日志器.
func WithSubscriberLogger(log logger.Logger) SubscriberOption {
	return func(o *subscriberOptions) {
		o.logger = logger.WithComponent(log, "PubSub")
	}
}

// WithConsumerGroup 设置消费者组名和消费者名称.
func WithConsumerGroup(group, consumer string) SubscriberOption {
	return func(o *subscriberOptions) {
		o.groupID = group
		o.consumer = consumer
	}
}

// WithBlock 设置是否使用阻塞读取（默认 true）.
func WithBlock(block bool) SubscriberOption {
	return func(o *subscriberOptions) {
		o.block = block
	}
}
