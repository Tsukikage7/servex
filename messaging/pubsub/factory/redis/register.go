// Package redis registers the Redis pubsub factory.
package redis

import (
	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
	"github.com/Tsukikage7/servex/v2/messaging/pubsub/factory"
	"github.com/Tsukikage7/servex/v2/messaging/pubsub/redis"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

func init() {
	factory.MustRegisterPublisher("redis", func(cfg *factory.Config, log logger.Logger) (pubsub.Publisher, error) {
		return redis.NewPublisherFromConfig(cfg.Addr, cfg.Password, cfg.DB, log)
	})
	factory.MustRegisterSubscriber("redis", func(cfg *factory.Config, group string, log logger.Logger) (pubsub.Subscriber, error) {
		return redis.NewSubscriberFromConfig(cfg.Addr, cfg.Password, cfg.DB, group, "", log)
	})
}
