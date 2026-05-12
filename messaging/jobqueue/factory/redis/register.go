// Package redis 注册 Redis jobqueue store 工厂.
package redis

import (
	"github.com/Tsukikage7/servex/v2/messaging/jobqueue"
	"github.com/Tsukikage7/servex/v2/messaging/jobqueue/factory"
	"github.com/Tsukikage7/servex/v2/messaging/jobqueue/redis"
)

func init() {
	factory.MustRegisterStore("redis", func(cfg *factory.StoreConfig) (jobqueue.Store, error) {
		return redis.NewStoreFromConfig(cfg.Addr, cfg.Password, cfg.DB, cfg.Prefix)
	})
}
