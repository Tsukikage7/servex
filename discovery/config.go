package discovery

import "time"

// TypeConsul 表示 Consul 服务发现类型.
const TypeConsul = "consul"

// TypeEtcd 表示 etcd 服务发现类型.
const TypeEtcd = "etcd"

// TypeNacos 表示 Nacos 服务发现类型.
const TypeNacos = "nacos"

const (
	// ProtocolHTTP 表示 HTTP 协议.
	ProtocolHTTP = "http"
	// ProtocolGRPC 表示 gRPC 协议.
	ProtocolGRPC = "grpc"
)

const (
	defaultHealthCheckInterval        = "10s"
	defaultHealthCheckTimeout         = "3s"
	defaultHealthCheckDeregisterAfter = "30s"
	defaultHealthCheckHTTPPath        = "/healthz"
)

// DefaultVersion 是默认服务版本.
const DefaultVersion = "1.0.0"

// Config 表示服务发现配置.
type Config struct {
	Type     string        `json:"type" toml:"type" yaml:"type" mapstructure:"type"`
	Addr     string        `json:"addr" toml:"addr" yaml:"addr" mapstructure:"addr"`
	Services ServiceConfig `json:"services" toml:"services" yaml:"services" mapstructure:"services"`

	// etcd 专用配置字段（Type == TypeEtcd 时生效）.
	EtcdEndpoints   []string      `json:"etcd_endpoints" toml:"etcd_endpoints" yaml:"etcd_endpoints" mapstructure:"etcd_endpoints"`
	EtcdDialTimeout time.Duration `json:"etcd_dial_timeout" toml:"etcd_dial_timeout" yaml:"etcd_dial_timeout" mapstructure:"etcd_dial_timeout"`

	// Nacos 专用配置字段（Type == TypeNacos 时生效）.
	NacosEndpoints   []string `json:"nacos_endpoints" toml:"nacos_endpoints" yaml:"nacos_endpoints" mapstructure:"nacos_endpoints"`
	NacosNamespaceID string   `json:"nacos_namespace_id" toml:"nacos_namespace_id" yaml:"nacos_namespace_id" mapstructure:"nacos_namespace_id"`
	NacosGroupName   string   `json:"nacos_group_name" toml:"nacos_group_name" yaml:"nacos_group_name" mapstructure:"nacos_group_name"`
}

// ServiceMetaConfig 表示服务元数据配置.
type ServiceMetaConfig struct {
	Version  string   `json:"version" toml:"version" yaml:"version" mapstructure:"version"`
	Protocol string   `json:"protocol" toml:"protocol" yaml:"protocol" mapstructure:"protocol"`
	Tags     []string `json:"tags" toml:"tags" yaml:"tags" mapstructure:"tags"`
}

// ServiceConfig 包含协议特定的服务配置.
type ServiceConfig struct {
	HTTP ServiceMetaConfig `json:"http" toml:"http" yaml:"http" mapstructure:"http"`
	GRPC ServiceMetaConfig `json:"grpc" toml:"grpc" yaml:"grpc" mapstructure:"grpc"`
}

// Validate 验证配置有效性.
func (c *Config) Validate() error {
	if c == nil {
		return ErrNilConfig
	}
	if c.Type == "" {
		return ErrEmptyType
	}
	if c.Type != TypeConsul && c.Type != TypeEtcd && c.Type != TypeNacos {
		return ErrUnsupportedType
	}
	return nil
}

// SetDefaults 设置配置的默认值.
func (c *Config) SetDefaults() {
	if c.Services.HTTP.Version == "" {
		c.Services.HTTP.Version = DefaultVersion
	}
	if c.Services.HTTP.Protocol == "" {
		c.Services.HTTP.Protocol = ProtocolHTTP
	}
	if len(c.Services.HTTP.Tags) == 0 {
		c.Services.HTTP.Tags = []string{"http", "v1"}
	}

	if c.Services.GRPC.Version == "" {
		c.Services.GRPC.Version = DefaultVersion
	}
	if c.Services.GRPC.Protocol == "" {
		c.Services.GRPC.Protocol = ProtocolGRPC
	}
	if len(c.Services.GRPC.Tags) == 0 {
		c.Services.GRPC.Tags = []string{"grpc", "v1"}
	}
}

// GetServiceConfig 返回指定协议的服务配置.
func (c *Config) GetServiceConfig(protocol string) ServiceMetaConfig {
	switch protocol {
	case ProtocolHTTP:
		return c.Services.HTTP
	case ProtocolGRPC:
		return c.Services.GRPC
	default:
		return ServiceMetaConfig{}
	}
}
