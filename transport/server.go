// Package transport 提供传输层抽象.
package transport

import (
	"context"
	"time"
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
	Runtime      RuntimeConfig `json:"runtime" yaml:"runtime" mapstructure:"runtime"`
	Logging      LoggingConfig `json:"logging" yaml:"logging" mapstructure:"logging"`
}

// GRPCConfig gRPC 服务器配置.
type GRPCConfig struct {
	Name             string        `json:"name" yaml:"name" mapstructure:"name"`
	Addr             string        `json:"addr" yaml:"addr" mapstructure:"addr"`
	EnableReflection *bool         `json:"enable_reflection" yaml:"enable_reflection" mapstructure:"enable_reflection"`
	KeepaliveTime    time.Duration `json:"keepalive_time" yaml:"keepalive_time" mapstructure:"keepalive_time"`
	KeepaliveTimeout time.Duration `json:"keepalive_timeout" yaml:"keepalive_timeout" mapstructure:"keepalive_timeout"`
	Runtime          RuntimeConfig `json:"runtime" yaml:"runtime" mapstructure:"runtime"`
	Logging          LoggingConfig `json:"logging" yaml:"logging" mapstructure:"logging"`
}

// RuntimeConfig 控制服务器运行时行为。
type RuntimeConfig struct {
	Recovery bool `json:"recovery" yaml:"recovery" mapstructure:"recovery"`
	Response bool `json:"response" yaml:"response" mapstructure:"response"`
}

// LoggingConfig 控制请求日志。
type LoggingConfig struct {
	Enabled   bool     `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	SkipPaths []string `json:"skip_paths" yaml:"skip_paths" mapstructure:"skip_paths"`
}

// TracingConfig 控制请求链路追踪中间件。
//
// 该配置只决定服务器是否注入 trace 中间件；TracerProvider / exporter 的生命周期
// 应由应用启动阶段显式初始化，避免服务器构造函数隐式连接外部系统。
type TracingConfig struct {
	Enabled   bool     `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	Service   string   `json:"service" yaml:"service" mapstructure:"service"`
	SkipPaths []string `json:"skip_paths" yaml:"skip_paths" mapstructure:"skip_paths"`
}

// MetricsConfig 控制 Prometheus 指标暴露。
type MetricsConfig struct {
	Enabled     bool   `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	Path        string `json:"path" yaml:"path" mapstructure:"path"`
	Namespace   string `json:"namespace" yaml:"namespace" mapstructure:"namespace"`
	ServiceName string `json:"service_name" yaml:"service_name" mapstructure:"service_name"`
}

// CORSConfig 控制 HTTP CORS 中间件。
type CORSConfig struct {
	Enabled          bool     `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	AllowOrigins     []string `json:"allow_origins" yaml:"allow_origins" mapstructure:"allow_origins"`
	AllowMethods     []string `json:"allow_methods" yaml:"allow_methods" mapstructure:"allow_methods"`
	AllowHeaders     []string `json:"allow_headers" yaml:"allow_headers" mapstructure:"allow_headers"`
	ExposeHeaders    []string `json:"expose_headers" yaml:"expose_headers" mapstructure:"expose_headers"`
	AllowCredentials bool     `json:"allow_credentials" yaml:"allow_credentials" mapstructure:"allow_credentials"`
	MaxAge           int      `json:"max_age" yaml:"max_age" mapstructure:"max_age"`
}

// GatewayConfig Gateway 服务器配置.
type GatewayConfig struct {
	Name          string        `json:"name" yaml:"name" mapstructure:"name"`
	Version       string        `json:"version" yaml:"version" mapstructure:"version"`
	GRPC          GRPCConfig    `json:"grpc" yaml:"grpc" mapstructure:"grpc"`
	HTTP          HTTPConfig    `json:"http" yaml:"http" mapstructure:"http"`
	Runtime       RuntimeConfig `json:"runtime" yaml:"runtime" mapstructure:"runtime"`
	Logging       LoggingConfig `json:"logging" yaml:"logging" mapstructure:"logging"`
	Tracing       TracingConfig `json:"tracing" yaml:"tracing" mapstructure:"tracing"`
	Metrics       MetricsConfig `json:"metrics" yaml:"metrics" mapstructure:"metrics"`
	CORS          CORSConfig    `json:"cors" yaml:"cors" mapstructure:"cors"`
	HealthTimeout time.Duration `json:"health_timeout" yaml:"health_timeout" mapstructure:"health_timeout"`
}
