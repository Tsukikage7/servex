// Package rabbitmq 注册 RabbitMQ jobqueue store 工厂.
package rabbitmq

import (
	"github.com/Tsukikage7/servex/v2/messaging/jobqueue"
	"github.com/Tsukikage7/servex/v2/messaging/jobqueue/factory"
	"github.com/Tsukikage7/servex/v2/messaging/jobqueue/rabbitmq"
)

func init() {
	factory.MustRegisterStore("rabbitmq", func(cfg *factory.StoreConfig) (jobqueue.Store, error) {
		return rabbitmq.NewStoreFromConfig(cfg.URL)
	})
}
