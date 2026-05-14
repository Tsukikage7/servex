// Package kafka registers the Kafka pubsub factory.
package kafka

import (
	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
	"github.com/Tsukikage7/servex/v2/messaging/pubsub/factory"
	"github.com/Tsukikage7/servex/v2/messaging/pubsub/kafka"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

func init() {
	factory.MustRegisterPublisher("kafka", func(cfg *factory.Config, log logger.Logger) (pubsub.Publisher, error) {
		return kafka.NewPublisherFromConfig(cfg.Brokers, log)
	})
	factory.MustRegisterSubscriber("kafka", func(cfg *factory.Config, group string, log logger.Logger) (pubsub.Subscriber, error) {
		return kafka.NewSubscriberFromConfig(cfg.Brokers, group, log)
	})
}
