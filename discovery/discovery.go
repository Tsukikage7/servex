// Package discovery 提供服务发现功能.
package discovery

import (
	"context"

	"github.com/Tsukikage7/servex/v2/transport"
)

// Discovery 定义服务发现接口.
type Discovery interface {
	// Register 注册服务实例，返回服务 ID.
	Register(ctx context.Context, serviceName, address string) (string, error)

	// RegisterWithProtocol 根据协议注册服务实例，返回服务 ID.
	RegisterWithProtocol(ctx context.Context, serviceName, address, protocol string) (string, error)

	// RegisterWithHealthEndpoint 使用指定的健康检查端点注册服务.
	// 当 healthEndpoint 为 nil 时使用默认 TCP 端口检查.
	RegisterWithHealthEndpoint(ctx context.Context, serviceName, address, protocol string, healthEndpoint *transport.HealthEndpoint) (string, error)

	// Unregister 注销服务实例.
	Unregister(ctx context.Context, serviceID string) error

	// Discover 发现服务实例.
	Discover(ctx context.Context, serviceName string) ([]string, error)

	// Close 关闭服务发现连接.
	Close() error
}
