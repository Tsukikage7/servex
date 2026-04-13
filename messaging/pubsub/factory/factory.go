// Package factory 提供根据配置创建 pubsub.Publisher 和 pubsub.Subscriber 的工厂方法.
//
// 该包解决了 pubsub 核心包与各 driver 子包之间的循环依赖问题：
// pubsub/kafka、pubsub/rabbitmq、pubsub/redis 均依赖 pubsub（获取 Message/Publisher/Subscriber），
// 因此工厂逻辑必须放在独立包中，而非 pubsub 本身.
package factory

import (
	"errors"
	"fmt"

	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
	"github.com/Tsukikage7/servex/v2/messaging/pubsub/kafka"
	"github.com/Tsukikage7/servex/v2/messaging/pubsub/rabbitmq"
	"github.com/Tsukikage7/servex/v2/messaging/pubsub/redis"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Config 配置 Pub/Sub 连接.
type Config struct {
	Type string `json:"type" yaml:"type"` // "kafka", "rabbitmq", "redis"

	// Kafka
	Brokers []string `json:"brokers" yaml:"brokers"`

	// RabbitMQ
	URL string `json:"url" yaml:"url"` // amqp://user:pass@host:port/vhost

	// Redis
	Addr     string `json:"addr"     yaml:"addr"`
	Password string `json:"-" yaml:"password"`
	DB       int    `json:"db"       yaml:"db"`
}

var (
	errNilConfig = errors.New("pubsub: config 不能为空")
	errEmptyType = errors.New("pubsub: type 不能为空")
)

// NewPublisher 根据 Config 创建 Publisher.
func NewPublisher(cfg *Config, log logger.Logger) (pubsub.Publisher, error) {
	if cfg == nil {
		return nil, errNilConfig
	}
	switch cfg.Type {
	case "":
		return nil, errEmptyType
	case "kafka":
		return kafka.NewPublisherFromConfig(cfg.Brokers, log)
	case "rabbitmq":
		return rabbitmq.NewPublisherFromConfig(cfg.URL, log)
	case "redis":
		return redis.NewPublisherFromConfig(cfg.Addr, cfg.Password, cfg.DB, log)
	default:
		return nil, fmt.Errorf("pubsub: 不支持的类型 %q", cfg.Type)
	}
}

// NewSubscriber 根据 Config 创建 Subscriber. group 用于 Kafka consumer group 和 Redis consumer group.
func NewSubscriber(cfg *Config, group string, log logger.Logger) (pubsub.Subscriber, error) {
	if cfg == nil {
		return nil, errNilConfig
	}
	switch cfg.Type {
	case "":
		return nil, errEmptyType
	case "kafka":
		return kafka.NewSubscriberFromConfig(cfg.Brokers, group, log)
	case "rabbitmq":
		return rabbitmq.NewSubscriberFromConfig(cfg.URL, log)
	case "redis":
		return redis.NewSubscriberFromConfig(cfg.Addr, cfg.Password, cfg.DB, group, "", log)
	default:
		return nil, fmt.Errorf("pubsub: 不支持的类型 %q", cfg.Type)
	}
}
