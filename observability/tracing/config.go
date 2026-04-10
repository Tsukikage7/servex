// Package tracing 提供分布式链路追踪功能.
package tracing

// TracingConfig 链路追踪配置.
type TracingConfig struct {
	// Enabled 是否启用链路追踪
	Enabled bool `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	// OTLP OTLP配置
	OTLP *OTLPConfig `json:"otlp" yaml:"otlp" mapstructure:"otlp"`
	// SamplingRate 采样率 (0.0-1.0)
	SamplingRate float64 `json:"sampling_rate" yaml:"sampling_rate" mapstructure:"sampling_rate"`
}

// OTLPConfig OTLP配置.
type OTLPConfig struct {
	// Endpoint OTLP Collector端点
	Endpoint string `json:"endpoint" yaml:"endpoint" mapstructure:"endpoint"`
	// Headers 请求头[可选]
	Headers map[string]string `json:"headers" yaml:"headers" mapstructure:"headers"`
	// Insecure 是否使用 HTTP（不加密），默认 true 以兼容旧行为；生产环境建议设为 false
	Insecure bool `json:"insecure" yaml:"insecure" mapstructure:"insecure"`
}
