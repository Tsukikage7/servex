package cache

import "github.com/Tsukikage7/servex/v2/observability/logger"

// Factory 创建指定类型的缓存实例.
type Factory func(*Config, logger.Logger) (Cache, error)

var factories = map[string]Factory{
	TypeMemory: NewMemoryCache,
}

// Register 注册缓存实现.
//
// Driver 子包应在 init 中调用 Register。核心 cache 包不直接导入具体后端，
// 避免使用缓存抽象时被动引入 Redis 等可选依赖.
func Register(cacheType string, factory Factory) {
	if cacheType == "" || factory == nil {
		return
	}
	factories[cacheType] = factory
}

// NewCache 创建缓存实例.
// logger 是必需参数，不能为 nil.
func NewCache(config *Config, log logger.Logger) (Cache, error) {
	if log == nil {
		return nil, ErrNilLogger
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	config.ApplyDefaults()

	factory, ok := factories[config.Type]
	if !ok {
		return nil, ErrUnsupported
	}
	return factory(config, log)
}

// MustNewCache 创建缓存实例，失败时 panic.
func MustNewCache(config *Config, log logger.Logger) Cache {
	cache, err := NewCache(config, log)
	if err != nil {
		panic(err)
	}
	return cache
}
