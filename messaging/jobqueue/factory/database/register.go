// Package database 注册 database jobqueue store 工厂.
package database

import (
	"github.com/Tsukikage7/servex/v2/messaging/jobqueue"
	"github.com/Tsukikage7/servex/v2/messaging/jobqueue/database"
	"github.com/Tsukikage7/servex/v2/messaging/jobqueue/factory"
)

func init() {
	factory.MustRegisterStore("database", func(cfg *factory.StoreConfig) (jobqueue.Store, error) {
		return database.NewStoreFromConfig(cfg.Driver, cfg.DSN, cfg.Table)
	})
}
