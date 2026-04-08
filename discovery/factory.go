package discovery

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/Tsukikage7/servex/observability/logger"
)

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

	switch config.Type {
	case TypeConsul:
		return newConsulDiscovery(config, log)
	case TypeEtcd:
		return newEtcdDiscovery(config, log)
	case TypeNacos:
		return newNacosDiscovery(config, log)
	default:
		return nil, ErrUnsupportedType
	}
}

// MustNewDiscovery 创建服务发现实例，失败时 panic.
func MustNewDiscovery(config *Config, log logger.Logger) Discovery {
	d, err := NewDiscovery(config, log)
	if err != nil {
		panic(err)
	}
	return d
}

// GenerateServiceID 生成唯一的服务ID.
func GenerateServiceID(serviceName string) string {
	if serviceName == "" {
		serviceName = "unknown"
	}

	// 创建随机数生成器
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 生成随机数
	randomNum := r.Intn(999999)

	// 获取时间戳
	timestamp := time.Now().Unix()

	return fmt.Sprintf("%s-%d%d", serviceName, randomNum, timestamp)
}
