// Package tracing 提供分布式链路追踪功能.
package tracing

// TracingConfig 链路追踪配置.
type TracingConfig struct {
	// Enabled 是否启用链路追踪
	Enabled bool `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	// ServiceName 服务名称
	ServiceName string `json:"service_name" yaml:"service_name" mapstructure:"service_name"`
	// ServiceVersion 服务版本
	ServiceVersion string `json:"service_version" yaml:"service_version" mapstructure:"service_version"`
	// OTLP OTLP配置
	OTLP *OTLPConfig `json:"otlp" yaml:"otlp" mapstructure:"otlp"`
	// SamplingRate 采样率 (0.0-1.0)
	SamplingRate float64 `json:"sampling_rate" yaml:"sampling_rate" mapstructure:"sampling_rate"`
}

// OTLPConfig OTLP配置.
type OTLPConfig struct {
	// Endpoint OTLP Collector端点
	Endpoint string `json:"endpoint" yaml:"endpoint" mapstructure:"endpoint"`
	// Protocol 传输协议，"http"（默认）或 "grpc"
	Protocol string `json:"protocol" yaml:"protocol" mapstructure:"protocol"`
	// Headers 请求头[可选]
	Headers map[string]string `json:"headers" yaml:"headers" mapstructure:"headers"`
	// Insecure 是否使用 HTTP（不加密），默认 true 以兼容旧行为；生产环境建议设为 false
	Insecure bool `json:"insecure" yaml:"insecure" mapstructure:"insecure"`
}
