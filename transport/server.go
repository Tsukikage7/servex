// Package transport 提供传输层抽象.
package transport

import (
	"context"
	"time"

	"github.com/Tsukikage7/servex/transport/health"
)

// Server 服务器接口.
type Server interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Name() string
	Addr() string
}

// HealthCheckType 健康检查类型.
type HealthCheckType string

const (
	// HealthCheckTypeTCP TCP 健康检查类型.
	HealthCheckTypeTCP HealthCheckType = "tcp"
	// HealthCheckTypeHTTP HTTP 健康检查类型.
	HealthCheckTypeHTTP HealthCheckType = "http"
	// HealthCheckTypeGRPC gRPC 健康检查类型.
	HealthCheckTypeGRPC HealthCheckType = "grpc"
)

// HealthEndpoint 健康检查端点信息.
type HealthEndpoint struct {
	Type HealthCheckType
	Addr string
	Path string
}

// HealthCheckable 支持健康检查的服务器.
type HealthCheckable interface {
	Server
	Health() *health.Health
	HealthEndpoint() *HealthEndpoint
}

// HTTPConfig HTTP 服务器配置.
type HTTPConfig struct {
	Name         string        `json:"name" yaml:"name" mapstructure:"name"`
	Addr         string        `json:"addr" yaml:"addr" mapstructure:"addr"`
	ReadTimeout  time.Duration `json:"read_timeout" yaml:"read_timeout" mapstructure:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout" mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `json:"idle_timeout" yaml:"idle_timeout" mapstructure:"idle_timeout"`
	PublicPaths  []string      `json:"public_paths" yaml:"public_paths" mapstructure:"public_paths"`
}

// GRPCConfig gRPC 服务器配置.
type GRPCConfig struct {
	Name             string        `json:"name" yaml:"name" mapstructure:"name"`
	Addr             string        `json:"addr" yaml:"addr" mapstructure:"addr"`
	EnableReflection *bool        `json:"enable_reflection" yaml:"enable_reflection" mapstructure:"enable_reflection"`
	KeepaliveTime    time.Duration `json:"keepalive_time" yaml:"keepalive_time" mapstructure:"keepalive_time"`
	KeepaliveTimeout time.Duration `json:"keepalive_timeout" yaml:"keepalive_timeout" mapstructure:"keepalive_timeout"`
	PublicMethods    []string      `json:"public_methods" yaml:"public_methods" mapstructure:"public_methods"`
}

// GatewayConfig Gateway 服务器配置.
type GatewayConfig struct {
	Name          string        `json:"name" yaml:"name" mapstructure:"name"`
	GRPCAddr      string        `json:"grpc_addr" yaml:"grpc_addr" mapstructure:"grpc_addr"`
	HTTPAddr      string        `json:"http_addr" yaml:"http_addr" mapstructure:"http_addr"`
	PublicMethods []string      `json:"public_methods" yaml:"public_methods" mapstructure:"public_methods"`
	KeepaliveTime time.Duration `json:"keepalive_time" yaml:"keepalive_time" mapstructure:"keepalive_time"`
}
