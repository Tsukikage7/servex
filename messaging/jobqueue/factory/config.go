// Package factory 提供根据配置创建 jobqueue.Store 实例的工厂方法.
package factory

import (
	"errors"
	"fmt"

	"github.com/Tsukikage7/servex/v2/messaging/jobqueue"
	"github.com/Tsukikage7/servex/v2/messaging/jobqueue/database"
	"github.com/Tsukikage7/servex/v2/messaging/jobqueue/kafka"
	"github.com/Tsukikage7/servex/v2/messaging/jobqueue/rabbitmq"
	jqredis "github.com/Tsukikage7/servex/v2/messaging/jobqueue/redis"
)

// StoreConfig 配置任务存储后端.
type StoreConfig struct {
	Type string `json:"type" yaml:"type"` // "redis", "kafka", "rabbitmq", "database"

	// Redis
	Addr     string `json:"addr"     yaml:"addr"`
	Password string `json:"-" yaml:"password"`
	DB       int    `json:"db"       yaml:"db"`
	Prefix   string `json:"prefix"   yaml:"prefix"`

	// Kafka
	Brokers []string `json:"brokers" yaml:"brokers"`

	// RabbitMQ
	URL string `json:"url" yaml:"url"`

	// Database
	Driver string `json:"driver" yaml:"driver"` // "mysql", "postgres", "sqlite"
	DSN    string `json:"dsn"    yaml:"dsn"`
	Table  string `json:"table"  yaml:"table"`

	// EnableTracing 启用链路追踪.
	// TODO: 待各 driver（redis/kafka/rabbitmq/database）OTEL 集成库成熟后启用
	EnableTracing bool `json:"enable_tracing" yaml:"enable_tracing" mapstructure:"enable_tracing"`
}

// NewStore 根据 StoreConfig 创建对应的 jobqueue.Store 实例.
func NewStore(cfg *StoreConfig) (jobqueue.Store, error) {
	if cfg == nil {
		return nil, errors.New("jobqueue/factory: StoreConfig 不能为空")
	}
	switch cfg.Type {
	case "redis":
		return jqredis.NewStoreFromConfig(cfg.Addr, cfg.Password, cfg.DB, cfg.Prefix)
	case "kafka":
		return kafka.NewStoreFromConfig(cfg.Brokers, cfg.Prefix)
	case "rabbitmq":
		return rabbitmq.NewStoreFromConfig(cfg.URL)
	case "database":
		return database.NewStoreFromConfig(cfg.Driver, cfg.DSN, cfg.Table)
	default:
		return nil, fmt.Errorf("jobqueue/factory: 不支持的存储类型 %q", cfg.Type)
	}
}
