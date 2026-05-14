package discovery

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Factory 创建指定类型的服务发现实例.
type Factory func(*Config, logger.Logger) (Discovery, error)

var factories = struct {
	sync.RWMutex
	creators map[string]Factory
}{
	creators: make(map[string]Factory),
}

// NewDiscovery 创建一个新的服务发现实例.
func NewDiscovery(config *Config, log logger.Logger) (Discovery, error) {
	if config == nil {
		return nil, ErrNilConfig
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// 设置默认值
	config.SetDefaults()

	factory, ok := lookupFactory(config.Type)
	if !ok {
		return nil, ErrUnsupportedType
	}
	return factory(config, log)
}

// MustNewDiscovery 创建服务发现实例，失败时 panic.
func MustNewDiscovery(config *Config, log logger.Logger) Discovery {
	d, err := NewDiscovery(config, log)
	if err != nil {
		panic(err)
	}
	return d
}

// Register 注册服务发现实现.
//
// 具体 provider 子包在 init 中调用 Register。根 discovery 包不直接导入
// Consul/etcd/Nacos SDK，避免仅使用抽象能力时被动拉入所有 provider 依赖.
func Register(discoveryType string, factory Factory) error {
	discoveryType = normalizeType(discoveryType)
	if discoveryType == "" {
		return ErrEmptyType
	}
	if factory == nil {
		return fmt.Errorf("discovery: factory 不能为空")
	}

	factories.Lock()
	defer factories.Unlock()
	if _, exists := factories.creators[discoveryType]; exists {
		return fmt.Errorf("discovery: 类型 %q 已注册", discoveryType)
	}
	factories.creators[discoveryType] = factory
	return nil
}

// MustRegister 注册服务发现实现，失败时 panic.
func MustRegister(discoveryType string, factory Factory) {
	if err := Register(discoveryType, factory); err != nil {
		panic(err)
	}
}

func lookupFactory(discoveryType string) (Factory, bool) {
	factories.RLock()
	defer factories.RUnlock()
	factory, ok := factories.creators[normalizeType(discoveryType)]
	return factory, ok
}

func normalizeType(discoveryType string) string {
	return strings.ToLower(strings.TrimSpace(discoveryType))
}

// GenerateServiceID 生成唯一的服务 ID.
func GenerateServiceID(serviceName string) string {
	if serviceName == "" {
		serviceName = "unknown"
	}
	return serviceName + "-" + randomID()
}

func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
