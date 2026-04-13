package kafka

import "github.com/Tsukikage7/servex/v2/observability/logger"

type publisherOptions struct {
	logger logger.Logger
}

// PublisherOption 配置 Kafka Publisher.
type PublisherOption func(*publisherOptions)

// WithPublisherLogger 设置日志器.
func WithPublisherLogger(log logger.Logger) PublisherOption {
	return func(o *publisherOptions) {
		o.logger = log
	}
}

type subscriberOptions struct {
	logger logger.Logger
}

// SubscriberOption 配置 Kafka Subscriber.
type SubscriberOption func(*subscriberOptions)

// WithSubscriberLogger 设置日志器.
func WithSubscriberLogger(log logger.Logger) SubscriberOption {
	return func(o *subscriberOptions) {
		o.logger = log
	}
}
