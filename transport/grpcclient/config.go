package grpcclient

import (
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/v2/middleware/circuitbreaker"
	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/observability/metrics"
	tlsx "github.com/Tsukikage7/servex/v2/transport/tls"
)

// Config gRPC 客户端配置.
type Config struct {
	ServiceName   string           `json:"service_name" yaml:"service_name" mapstructure:"service_name"`
	Addr          string           `json:"addr" yaml:"addr" mapstructure:"addr"`
	TLS           *tlsx.Config     `json:"tls" yaml:"tls" mapstructure:"tls"`
	Timeout       time.Duration    `json:"timeout" yaml:"timeout" mapstructure:"timeout"`
	Retry         *RetryConfig     `json:"retry" yaml:"retry" mapstructure:"retry"`
	Balancer      string           `json:"balancer" yaml:"balancer" mapstructure:"balancer"` // round_robin | pick_first
	Keepalive     *KeepaliveConfig `json:"keepalive" yaml:"keepalive" mapstructure:"keepalive"`
	EnableTracing bool             `json:"enable_tracing" yaml:"enable_tracing" mapstructure:"enable_tracing"`
	EnableMetrics bool             `json:"enable_metrics" yaml:"enable_metrics" mapstructure:"enable_metrics"`
}

// RetryConfig 重试配置.
type RetryConfig struct {
	MaxAttempts int           `json:"max_attempts" yaml:"max_attempts" mapstructure:"max_attempts"`
	Backoff     time.Duration `json:"backoff" yaml:"backoff" mapstructure:"backoff"`
}

// KeepaliveConfig Keepalive 配置.
type KeepaliveConfig struct {
	Time    time.Duration `json:"time" yaml:"time" mapstructure:"time"`
	Timeout time.Duration `json:"timeout" yaml:"timeout" mapstructure:"timeout"`
}

// NewFromConfig 从配置创建 gRPC 客户端.
//
// 不使用服务发现，直接连接 cfg.Addr. 如需服务发现请使用 New.
func NewFromConfig(cfg *Config, additionalOpts ...Option) (*Client, error) {
	if cfg.Addr == "" {
		return nil, ErrMissingAddr
	}

	var opts []Option

	if cfg.ServiceName != "" {
		opts = append(opts, WithName(cfg.ServiceName))
	}

	if cfg.Timeout > 0 {
		opts = append(opts, WithTimeout(cfg.Timeout))
	}

	if cfg.TLS != nil {
		tlsCfg, err := tlsx.NewClientTLSConfig(cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("grpc client: failed to create TLS config: %w", err)
		}
		opts = append(opts, WithTLS(tlsCfg))
	}

	if cfg.Retry != nil && cfg.Retry.MaxAttempts > 0 {
		backoff := cfg.Retry.Backoff
		if backoff == 0 {
			backoff = 100 * time.Millisecond
		}
		opts = append(opts, WithRetry(cfg.Retry.MaxAttempts, backoff))
	}

	if cfg.Balancer != "" {
		opts = append(opts, WithBalancer(cfg.Balancer))
	}

	if cfg.Keepalive != nil {
		opts = append(opts, WithKeepalive(cfg.Keepalive.Time, cfg.Keepalive.Timeout))
	}

	if cfg.EnableTracing {
		name := cfg.ServiceName
		if name == "" {
			name = "grpcclient"
		}
		opts = append(opts, WithTracing(name))
	}

	if cfg.EnableMetrics {
		// EnableMetrics flag; actual collector must be passed via additionalOpts
		// or we can check if one is provided
	}

	opts = append(opts, additionalOpts...)

	return newDirect(cfg.Addr, opts...)
}

// newDirect 直接连接指定地址创建客户端不走服务发现.
func newDirect(addr string, opts ...Option) (*Client, error) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	if o.logger == nil {
		o.logger = logger.Nop()
	}
	o.logger = logger.WithComponent(o.logger, "gRPC")

	dialOpts := buildDialOptions(o)

	conn, err := dialWithTimeout(addr, o.timeout, dialOpts...)
	if err != nil {
		return nil, ErrConnectionFailed.WithCause(err)
	}

	o.logger.With(logger.String("addr", addr)).Info("客户端直连初始化成功")

	return &Client{
		conn: conn,
		opts: o,
	}, nil
}

// metricsCollectorFromConfig 是辅助函数，用于检查 additionalOpts 中是否有 metrics collector.
func metricsCollectorFromConfig(cfg *Config, collector *metrics.PrometheusCollector) []Option {
	if cfg.EnableMetrics && collector != nil {
		return []Option{WithMetrics(collector)}
	}
	return nil
}

// NewFromConfigWithMetrics 从配置创建客户端，同时传入 metrics collector.
func NewFromConfigWithMetrics(cfg *Config, collector *metrics.PrometheusCollector, additionalOpts ...Option) (*Client, error) {
	extra := metricsCollectorFromConfig(cfg, collector)
	return NewFromConfig(cfg, append(extra, additionalOpts...)...)
}

// NewFromConfigWithDeps 从配置创建客户端，同时传入 metrics collector 和 circuit breaker.
func NewFromConfigWithDeps(cfg *Config, collector *metrics.PrometheusCollector, cb circuitbreaker.CircuitBreaker, additionalOpts ...Option) (*Client, error) {
	var extra []Option
	if cfg.EnableMetrics && collector != nil {
		extra = append(extra, WithMetrics(collector))
	}
	if cb != nil {
		extra = append(extra, WithCircuitBreaker(cb))
	}
	return NewFromConfig(cfg, append(extra, additionalOpts...)...)
}
