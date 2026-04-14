// Package discovery 提供服务发现功能.
package discovery

import (
	"context"
	"strings"
	"sync"

	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/transport"
)

// ServiceInfo 表示服务信息.
type ServiceInfo struct {
	Name           string
	Addr           string
	Protocol       string
	HealthEndpoint *transport.HealthEndpoint
}

// ServiceRegistry 管理多个服务的注册和注销.
type ServiceRegistry struct {
	discovery     Discovery
	logger        logger.Logger
	advertiseAddr string
	services      []ServiceInfo
	serviceIDs    []string
	mu            sync.Mutex
}

// RegistryOption 是 ServiceRegistry 的可选配置.
type RegistryOption func(*ServiceRegistry)

// WithAdvertiseAddr 设置服务注册的通告地址.
// 当服务地址仅包含端口（如 ":8080"）时，会自动拼接此地址作为 host.
func WithAdvertiseAddr(addr string) RegistryOption {
	return func(r *ServiceRegistry) {
		r.advertiseAddr = addr
	}
}

// NewServiceRegistry 创建服务注册器.
func NewServiceRegistry(discovery Discovery, logger logger.Logger, opts ...RegistryOption) *ServiceRegistry {
	r := &ServiceRegistry{
		discovery:  discovery,
		logger:     logger,
		services:   make([]ServiceInfo, 0),
		serviceIDs: make([]string, 0),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// AddService 添加要注册的服务.
func (r *ServiceRegistry) AddService(name, addr, protocol string) *ServiceRegistry {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services = append(r.services, ServiceInfo{
		Name:     name,
		Addr:     r.resolveAddr(addr),
		Protocol: protocol,
	})
	return r
}

// AddGRPC 添加 gRPC 服务.
func (r *ServiceRegistry) AddGRPC(name, addr string) *ServiceRegistry {
	return r.AddService(name, addr, ProtocolGRPC)
}

// AddHTTP 添加 HTTP 服务.
func (r *ServiceRegistry) AddHTTP(name, addr string) *ServiceRegistry {
	return r.AddService(name, addr, ProtocolHTTP)
}

// AddServer 从 Server 添加服务，自动检测健康检查端点.
// 如果 Server 实现了 HealthCheckable 接口，将自动提取健康检查配置.
func (r *ServiceRegistry) AddServer(name string, server transport.Server) *ServiceRegistry {
	r.mu.Lock()
	defer r.mu.Unlock()

	info := ServiceInfo{
		Name: name,
		Addr: r.resolveAddr(server.Addr()),
	}
	info.Protocol = r.inferProtocol(server)

	if hc, ok := server.(transport.HealthCheckable); ok {
		info.HealthEndpoint = hc.HealthEndpoint()
	}

	r.services = append(r.services, info)
	return r
}

// AddServerWithProtocol 从 Server 添加服务，指定协议类型.
func (r *ServiceRegistry) AddServerWithProtocol(name string, server transport.Server, protocol string) *ServiceRegistry {
	r.mu.Lock()
	defer r.mu.Unlock()

	info := ServiceInfo{
		Name:     name,
		Addr:     r.resolveAddr(server.Addr()),
		Protocol: protocol,
	}

	if hc, ok := server.(transport.HealthCheckable); ok {
		info.HealthEndpoint = hc.HealthEndpoint()
	}

	r.services = append(r.services, info)
	return r
}

func (r *ServiceRegistry) inferProtocol(server transport.Server) string {
	if hc, ok := server.(transport.HealthCheckable); ok {
		endpoint := hc.HealthEndpoint()
		if endpoint != nil {
			switch endpoint.Type {
			case transport.HealthCheckTypeGRPC:
				return ProtocolGRPC
			case transport.HealthCheckTypeHTTP:
				return ProtocolHTTP
			}
		}
	}

	name := server.Name()
	switch {
	case containsIgnoreCase(name, "grpc"):
		return ProtocolGRPC
	case containsIgnoreCase(name, "http"):
		return ProtocolHTTP
	case containsIgnoreCase(name, "gateway"):
		return ProtocolGRPC
	default:
		return ProtocolGRPC
	}
}

// RegisterAll 注册所有服务.
func (r *ServiceRegistry) RegisterAll(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, svc := range r.services {
		var serviceID string
		var err error

		if svc.HealthEndpoint != nil {
			serviceID, err = r.discovery.RegisterWithHealthEndpoint(ctx, svc.Name, svc.Addr, svc.Protocol, svc.HealthEndpoint)
		} else {
			serviceID, err = r.discovery.RegisterWithProtocol(ctx, svc.Name, svc.Addr, svc.Protocol)
		}

		if err != nil {
			r.logger.With(
				logger.String("serviceName", svc.Name),
				logger.String("addr", svc.Addr),
				logger.String("protocol", svc.Protocol),
				logger.Err(err),
			).Error("[Discovery] 注册服务失败")
			r.unregisterAllLocked(ctx)
			return err
		}
		r.serviceIDs = append(r.serviceIDs, serviceID)

		healthType := "TCP"
		if svc.HealthEndpoint != nil {
			healthType = string(svc.HealthEndpoint.Type)
		}
		r.logger.With(
			logger.String("serviceName", svc.Name),
			logger.String("addr", svc.Addr),
			logger.String("protocol", svc.Protocol),
			logger.String("healthCheckType", healthType),
			logger.String("serviceID", serviceID),
		).Info("[Discovery] 服务已注册")
	}
	return nil
}

// UnregisterAll 注销所有服务.
func (r *ServiceRegistry) UnregisterAll(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.unregisterAllLocked(ctx)
}

func (r *ServiceRegistry) unregisterAllLocked(ctx context.Context) error {
	var lastErr error
	for _, serviceID := range r.serviceIDs {
		if err := r.discovery.Unregister(ctx, serviceID); err != nil {
			r.logger.With(
				logger.String("serviceID", serviceID),
				logger.Err(err),
			).Error("[Discovery] 注销服务失败")
			lastErr = err
		} else {
			r.logger.With(logger.String("serviceID", serviceID)).Info("[Discovery] 服务已注销")
		}
	}
	r.serviceIDs = r.serviceIDs[:0]
	return lastErr
}

// AfterStartHook 返回服务启动后的注册钩子，可用于 HooksBuilder.AfterStart.
func (r *ServiceRegistry) AfterStartHook() func(ctx context.Context) error {
	return r.RegisterAll
}

// BeforeStopHook 返回服务停止前的注销钩子，可用于 HooksBuilder.BeforeStop.
func (r *ServiceRegistry) BeforeStopHook() func(ctx context.Context) error {
	return r.UnregisterAll
}

// resolveAddr 解析服务地址，如果地址仅包含端口（以 ":" 开头），则拼接 advertiseAddr.
func (r *ServiceRegistry) resolveAddr(addr string) string {
	if r.advertiseAddr != "" && strings.HasPrefix(addr, ":") {
		return r.advertiseAddr + addr
	}
	return addr
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
