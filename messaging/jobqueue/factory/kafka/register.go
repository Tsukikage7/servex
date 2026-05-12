// Package kafka 注册 Kafka jobqueue store 工厂.
package kafka

import (
	"github.com/Tsukikage7/servex/v2/messaging/jobqueue"
	"github.com/Tsukikage7/servex/v2/messaging/jobqueue/factory"
	"github.com/Tsukikage7/servex/v2/messaging/jobqueue/kafka"
)

func init() {
	factory.MustRegisterStore("kafka", func(cfg *factory.StoreConfig) (jobqueue.Store, error) {
		return kafka.NewStoreFromConfig(cfg.Brokers, cfg.Prefix)
	})
}
