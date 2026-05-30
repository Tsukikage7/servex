// Package observability 提供服务级可观测性配置结构和组件埋点开关.
package observability

import (
	"time"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

const (
	ExporterOTLP   = "otlp"
	ExporterStdout = "stdout"
)

const (
	ProtocolGRPC = "grpc"
	ProtocolHTTP = "http"
)

const (
	InstrumentationHTTPServer    = "http.server"
	InstrumentationHTTPClient    = "http.client"
	InstrumentationGRPCServer    = "grpc.server"
	InstrumentationGRPCClient    = "grpc.client"
	InstrumentationRedis         = "redis"
	InstrumentationRDBMS         = "rdbms"
	InstrumentationMongoDB       = "mongodb"
	InstrumentationClickHouse    = "clickhouse"
	InstrumentationNeo4j         = "neo4j"
	InstrumentationS3            = "s3"
	InstrumentationMinIO         = "minio"
	InstrumentationElasticsearch = "elasticsearch"
	InstrumentationMessaging     = "messaging"
	InstrumentationJobQueue      = "jobqueue"
	InstrumentationGraphQL       = "graphql"
	InstrumentationWebSocket     = "websocket"
	InstrumentationGoIM          = "goim"
	InstrumentationLLM           = "llm"
)

// Config 描述服务级可观测性配置.
type Config struct {
	Service          ServiceConfig         `json:"service" yaml:"service" mapstructure:"service"`
	Logger           logger.Config         `json:"logger" yaml:"logger" mapstructure:"logger"`
	Tracing          TracingConfig         `json:"tracing" yaml:"tracing" mapstructure:"tracing"`
	Metrics          MetricsConfig         `json:"metrics" yaml:"metrics" mapstructure:"metrics"`
	Instrumentations InstrumentationConfig `json:"instrumentations" yaml:"instrumentations" mapstructure:"instrumentations"`
}

// ServiceConfig 描述 OpenTelemetry resource 中的服务身份.
type ServiceConfig struct {
	Name        string `json:"name" yaml:"name" mapstructure:"name"`
	Version     string `json:"version" yaml:"version" mapstructure:"version"`
	Namespace   string `json:"namespace" yaml:"namespace" mapstructure:"namespace"`
	Environment string `json:"environment" yaml:"environment" mapstructure:"environment"`
}

// TracingConfig 描述链路追踪配置.
type TracingConfig struct {
	Enabled      bool                  `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	SamplingRate float64               `json:"sampling_rate" yaml:"sampling_rate" mapstructure:"sampling_rate"`
	Exporters    []TraceExporterConfig `json:"exporters" yaml:"exporters" mapstructure:"exporters"`
}

// TraceExporterConfig 描述 trace exporter.
type TraceExporterConfig struct {
	Type     string            `json:"type" yaml:"type" mapstructure:"type"`
	Endpoint string            `json:"endpoint" yaml:"endpoint" mapstructure:"endpoint"`
	Protocol string            `json:"protocol" yaml:"protocol" mapstructure:"protocol"`
	Headers  map[string]string `json:"headers" yaml:"headers" mapstructure:"headers"`
	Insecure bool              `json:"insecure" yaml:"insecure" mapstructure:"insecure"`
}

// MetricsConfig 描述指标配置.
type MetricsConfig struct {
	Enabled   bool                   `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	Interval  time.Duration          `json:"interval" yaml:"interval" mapstructure:"interval"`
	Exporters []MetricExporterConfig `json:"exporters" yaml:"exporters" mapstructure:"exporters"`
}

// MetricExporterConfig 描述 metric exporter.
type MetricExporterConfig struct {
	Type     string            `json:"type" yaml:"type" mapstructure:"type"`
	Endpoint string            `json:"endpoint" yaml:"endpoint" mapstructure:"endpoint"`
	Protocol string            `json:"protocol" yaml:"protocol" mapstructure:"protocol"`
	Headers  map[string]string `json:"headers" yaml:"headers" mapstructure:"headers"`
	Insecure bool              `json:"insecure" yaml:"insecure" mapstructure:"insecure"`
}

// InstrumentationConfig 描述组件埋点开关.
type InstrumentationConfig struct {
	DefaultEnabled bool            `json:"default_enabled" yaml:"default_enabled" mapstructure:"default_enabled"`
	Overrides      map[string]bool `json:"overrides" yaml:"overrides" mapstructure:"overrides"`
}

// DefaultConfig 返回可直接用于服务模板的默认配置.
func DefaultConfig(name, version string) Config {
	cfg := Config{
		Service: ServiceConfig{
			Name:    name,
			Version: version,
		},
		Logger: logger.Config{
			ServiceName: name,
			Level:       logger.LevelInfo,
			Format:      logger.FormatConsole,
			Output:      logger.OutputConsole,
		},
		Tracing: TracingConfig{
			SamplingRate: 1,
		},
		Metrics: MetricsConfig{
			Interval: time.Minute,
		},
		Instrumentations: InstrumentationConfig{
			DefaultEnabled: true,
		},
	}
	cfg.ApplyDefaults()
	return cfg
}

// ApplyDefaults 补齐配置默认值.
func (c *Config) ApplyDefaults() {
	if c.Service.Name == "" {
		c.Service.Name = "service"
	}
	if c.Service.Version == "" {
		c.Service.Version = "dev"
	}
	if c.Logger.ServiceName == "" {
		c.Logger.ServiceName = c.Service.Name
	}
	c.Logger.ApplyDefaults()
	if c.Tracing.SamplingRate <= 0 || c.Tracing.SamplingRate > 1 {
		c.Tracing.SamplingRate = 1
	}
	if c.Metrics.Interval <= 0 {
		c.Metrics.Interval = time.Minute
	}
	if !c.Instrumentations.DefaultEnabled && len(c.Instrumentations.Overrides) == 0 {
		c.Instrumentations.DefaultEnabled = true
	}
}

// Enabled 判断组件埋点是否启用.
func (c InstrumentationConfig) Enabled(name string) bool {
	if c.Overrides != nil {
		if enabled, ok := c.Overrides[name]; ok {
			return enabled
		}
	}
	return c.DefaultEnabled
}

// TraceEnabled 判断组件 trace 是否启用.
func (c Config) TraceEnabled(name string) bool {
	if !c.Tracing.Enabled {
		return false
	}
	return c.Instrumentations.Enabled(name)
}

// MetricsEnabled 判断组件 metrics 是否启用.
func (c Config) MetricsEnabled(name string) bool {
	if !c.Metrics.Enabled {
		return false
	}
	return c.Instrumentations.Enabled(name)
}
