// Package factory 提供根据配置创建 pubsub.Publisher 和 pubsub.Subscriber 的工厂方法.
//
// 该包解决了 pubsub 核心包与各 driver 子包之间的循环依赖问题：
// pubsub/kafka、pubsub/rabbitmq、pubsub/redis 均依赖 pubsub（获取 Message/Publisher/Subscriber），
// 因此工厂逻辑必须放在独立包中，而非 pubsub 本身.
package factory

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// PublisherCreator 根据 Config 创建 Publisher.
type PublisherCreator func(*Config, logger.Logger) (pubsub.Publisher, error)

// SubscriberCreator 根据 Config 创建 Subscriber.
type SubscriberCreator func(*Config, string, logger.Logger) (pubsub.Subscriber, error)

var registry = struct {
	sync.RWMutex
	publishers  map[string]PublisherCreator
	subscribers map[string]SubscriberCreator
}{
	publishers:  make(map[string]PublisherCreator),
	subscribers: make(map[string]SubscriberCreator),
}

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

	// EnableTracing 启用链路追踪（Publish 时自动注入 trace context 到消息 Headers）.
	EnableTracing bool `json:"enable_tracing" yaml:"enable_tracing" mapstructure:"enable_tracing"`
}

var (
	errNilConfig = errors.New("pubsub: config 不能为空")
	errEmptyType = errors.New("pubsub: type 不能为空")
)

// NewPublisher 根据 Config 创建 Publisher.
// 当 EnableTracing 为 true 时，自动使用 TracingPublisher 包装，Publish 时注入 trace context.
func NewPublisher(cfg *Config, log logger.Logger) (pubsub.Publisher, error) {
	if cfg == nil {
		return nil, errNilConfig
	}
	pubType := normalizeType(cfg.Type)
	if pubType == "" {
		return nil, errEmptyType
	}
	creator, ok := lookupPublisher(pubType)
	if !ok {
		return nil, fmt.Errorf("pubsub: 不支持的类型 %q", cfg.Type)
	}
	p, err := creator(cfg, log)
	if err != nil {
		return nil, err
	}
	if cfg.EnableTracing {
		p = pubsub.NewTracingPublisher(p)
	}
	return p, nil
}

// NewSubscriber 根据 Config 创建 Subscriber. group 用于 Kafka consumer group 和 Redis consumer group.
func NewSubscriber(cfg *Config, group string, log logger.Logger) (pubsub.Subscriber, error) {
	if cfg == nil {
		return nil, errNilConfig
	}
	pubType := normalizeType(cfg.Type)
	if pubType == "" {
		return nil, errEmptyType
	}
	creator, ok := lookupSubscriber(pubType)
	if !ok {
		return nil, fmt.Errorf("pubsub: 不支持的类型 %q", cfg.Type)
	}
	return creator(cfg, group, log)
}

// RegisterPublisher 注册 Publisher 创建器.
func RegisterPublisher(pubType string, creator PublisherCreator) error {
	pubType = normalizeType(pubType)
	if pubType == "" {
		return errEmptyType
	}
	if creator == nil {
		return errors.New("pubsub: publisher creator 不能为空")
	}

	registry.Lock()
	defer registry.Unlock()
	if _, exists := registry.publishers[pubType]; exists {
		return fmt.Errorf("pubsub: publisher 类型 %q 已注册", pubType)
	}
	registry.publishers[pubType] = creator
	return nil
}

// RegisterSubscriber 注册 Subscriber 创建器.
func RegisterSubscriber(pubType string, creator SubscriberCreator) error {
	pubType = normalizeType(pubType)
	if pubType == "" {
		return errEmptyType
	}
	if creator == nil {
		return errors.New("pubsub: subscriber creator 不能为空")
	}

	registry.Lock()
	defer registry.Unlock()
	if _, exists := registry.subscribers[pubType]; exists {
		return fmt.Errorf("pubsub: subscriber 类型 %q 已注册", pubType)
	}
	registry.subscribers[pubType] = creator
	return nil
}

// MustRegisterPublisher 注册 Publisher 创建器，失败时 panic.
func MustRegisterPublisher(pubType string, creator PublisherCreator) {
	if err := RegisterPublisher(pubType, creator); err != nil {
		panic(err)
	}
}

// MustRegisterSubscriber 注册 Subscriber 创建器，失败时 panic.
func MustRegisterSubscriber(pubType string, creator SubscriberCreator) {
	if err := RegisterSubscriber(pubType, creator); err != nil {
		panic(err)
	}
}

func lookupPublisher(pubType string) (PublisherCreator, bool) {
	registry.RLock()
	defer registry.RUnlock()
	creator, ok := registry.publishers[pubType]
	return creator, ok
}

func lookupSubscriber(pubType string) (SubscriberCreator, bool) {
	registry.RLock()
	defer registry.RUnlock()
	creator, ok := registry.subscribers[pubType]
	return creator, ok
}

func normalizeType(pubType string) string {
	return strings.ToLower(strings.TrimSpace(pubType))
}
