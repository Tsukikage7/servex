package rabbitmq

import "github.com/Tsukikage7/servex/v2/observability/logger"

type publisherOptions struct {
	logger       logger.Logger
	exchange     string
	exchangeType string
	durable      bool
	confirm      bool
}

// PublisherOption 配置 RabbitMQ Publisher.
type PublisherOption func(*publisherOptions)

// WithPublisherLogger 设置日志器.
func WithPublisherLogger(log logger.Logger) PublisherOption {
	return func(o *publisherOptions) {
		o.logger = logger.WithComponent(log, "RabbitMQ")
	}
}

// WithExchange 设置交换机名称和类型direct/fanout/topic.
func WithExchange(name, typ string) PublisherOption {
	return func(o *publisherOptions) {
		o.exchange = name
		o.exchangeType = typ
	}
}

// WithPublisherConfirm 开启发布确认.
func WithPublisherConfirm(enabled bool) PublisherOption {
	return func(o *publisherOptions) {
		o.confirm = enabled
	}
}

// WithPublisherDurable 设置交换机持久化.
func WithPublisherDurable(durable bool) PublisherOption {
	return func(o *publisherOptions) {
		o.durable = durable
	}
}

type subscriberOptions struct {
	logger        logger.Logger
	exchange      string
	exchangeType  string
	durable       bool
	autoAck       bool
	prefetchCount int
}

// SubscriberOption 配置 RabbitMQ Subscriber.
type SubscriberOption func(*subscriberOptions)

// WithSubscriberLogger 设置日志器.
func WithSubscriberLogger(log logger.Logger) SubscriberOption {
	return func(o *subscriberOptions) {
		o.logger = logger.WithComponent(log, "RabbitMQ")
	}
}

// WithSubscriberExchange 设置交换机名称和类型.
func WithSubscriberExchange(name, typ string) SubscriberOption {
	return func(o *subscriberOptions) {
		o.exchange = name
		o.exchangeType = typ
	}
}

// WithSubscriberDurable 设置队列持久化.
func WithSubscriberDurable(durable bool) SubscriberOption {
	return func(o *subscriberOptions) {
		o.durable = durable
	}
}

// WithAutoAck 设置是否自动确认消息.
func WithAutoAck(autoAck bool) SubscriberOption {
	return func(o *subscriberOptions) {
		o.autoAck = autoAck
	}
}

// WithPrefetchCount 设置预取消息数量.
func WithPrefetchCount(count int) SubscriberOption {
	return func(o *subscriberOptions) {
		o.prefetchCount = count
	}
}
