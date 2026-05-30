// Package factory 提供根据配置创建 jobqueue.Store 实例的工厂方法.
package factory

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Tsukikage7/servex/v2/messaging/jobqueue"
)

// StoreCreator 根据 StoreConfig 创建 jobqueue.Store.
type StoreCreator func(*StoreConfig) (jobqueue.Store, error)

var registry = struct {
	sync.RWMutex
	creators map[string]StoreCreator
}{
	creators: make(map[string]StoreCreator),
}

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

	// EnableTracing 启用链路追踪Enqueue 时自动注入 trace context 到 Job Headers.
	EnableTracing bool `json:"enable_tracing" yaml:"enable_tracing" mapstructure:"enable_tracing"`

	// TracerName 链路追踪 tracer 名称用于 Worker span，默认 "jobqueue".
	TracerName string `json:"tracer_name" yaml:"tracer_name" mapstructure:"tracer_name"`
}

// NewStore 根据 StoreConfig 创建对应的 jobqueue.Store 实例.
func NewStore(cfg *StoreConfig) (jobqueue.Store, error) {
	if cfg == nil {
		return nil, errors.New("jobqueue/factory: StoreConfig 不能为空")
	}
	storeType := normalizeType(cfg.Type)
	if storeType == "" {
		return nil, errors.New("jobqueue/factory: 存储类型不能为空")
	}
	creator, ok := lookup(storeType)
	if !ok {
		return nil, fmt.Errorf("jobqueue/factory: 不支持的存储类型 %q", cfg.Type)
	}
	return creator(cfg)
}

// RegisterStore 注册 Store 创建器.
//
// factory 包本身不导入 Redis/Kafka/RabbitMQ/Database 后端，避免业务仅使用
// factory 时被动拉入未使用的间接依赖。按需导入
// messaging/jobqueue/factory/<driver> 注册包即可启用对应类型。
func RegisterStore(storeType string, creator StoreCreator) error {
	storeType = normalizeType(storeType)
	if storeType == "" {
		return errors.New("jobqueue/factory: 存储类型不能为空")
	}
	if creator == nil {
		return errors.New("jobqueue/factory: StoreCreator 不能为空")
	}

	registry.Lock()
	defer registry.Unlock()
	if _, exists := registry.creators[storeType]; exists {
		return fmt.Errorf("jobqueue/factory: 存储类型 %q 已注册", storeType)
	}
	registry.creators[storeType] = creator
	return nil
}

// MustRegisterStore 注册 Store 创建器，失败时 panic.
func MustRegisterStore(storeType string, creator StoreCreator) {
	if err := RegisterStore(storeType, creator); err != nil {
		panic(err)
	}
}

func lookup(storeType string) (StoreCreator, bool) {
	registry.RLock()
	defer registry.RUnlock()
	creator, ok := registry.creators[storeType]
	return creator, ok
}

func normalizeType(storeType string) string {
	return strings.ToLower(strings.TrimSpace(storeType))
}

// NewClient 根据 StoreConfig 创建 Client.
// 当 EnableTracing 为 true 时，自动使用 TracingClient 包装.
func NewClient(cfg *StoreConfig) (jobqueue.Client, error) {
	store, err := NewStore(cfg)
	if err != nil {
		return nil, err
	}
	c := jobqueue.NewClient(store)
	if cfg.EnableTracing {
		return jobqueue.NewTracingClient(c), nil
	}
	return c, nil
}

// NewWorker 根据 StoreConfig 创建 Worker.
// 当 EnableTracing 为 true 时，自动使用 TracingWorker 包装.
func NewWorker(cfg *StoreConfig, opts ...jobqueue.WorkerOption) (jobqueue.Worker, error) {
	store, err := NewStore(cfg)
	if err != nil {
		return nil, err
	}
	w := jobqueue.NewWorker(store, opts...)
	if cfg.EnableTracing {
		tracerName := cfg.TracerName
		if tracerName == "" {
			tracerName = "jobqueue"
		}
		return jobqueue.NewTracingWorker(w, tracerName), nil
	}
	return w, nil
}
