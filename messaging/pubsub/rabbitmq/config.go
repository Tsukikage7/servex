package rabbitmq

import "github.com/Tsukikage7/servex/v2/observability/logger"

// NewPublisherFromConfig 根据 AMQP URL 创建 Publisher.
func NewPublisherFromConfig(url string, log logger.Logger) (*Publisher, error) {
	return NewPublisher(url, WithPublisherLogger(log))
}

// NewSubscriberFromConfig 根据 AMQP URL 创建 Subscriber.
func NewSubscriberFromConfig(url string, log logger.Logger) (*Subscriber, error) {
	return NewSubscriber(url, WithSubscriberLogger(log))
}
