// Package rabbitmq registers the RabbitMQ pubsub factory.
package rabbitmq

import (
	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
	"github.com/Tsukikage7/servex/v2/messaging/pubsub/factory"
	"github.com/Tsukikage7/servex/v2/messaging/pubsub/rabbitmq"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

func init() {
	factory.MustRegisterPublisher("rabbitmq", func(cfg *factory.Config, log logger.Logger) (pubsub.Publisher, error) {
		return rabbitmq.NewPublisherFromConfig(cfg.URL, log)
	})
	factory.MustRegisterSubscriber("rabbitmq", func(cfg *factory.Config, group string, log logger.Logger) (pubsub.Subscriber, error) {
		return rabbitmq.NewSubscriberFromConfig(cfg.URL, log)
	})
}
