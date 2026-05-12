package grpcclient

import (
	"crypto/tls"
	"time"

	"google.golang.org/grpc"

	"github.com/Tsukikage7/servex/v2/discovery"
	"github.com/Tsukikage7/servex/v2/middleware/circuitbreaker"
	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/observability/metrics"
)

// Option 配置选项函数.
type Option func(*options)

// options 客户端配置.
type options struct {
	name               string
	serviceName        string
	discovery          discovery.Discovery
	logger             logger.Logger
	interceptors       []grpc.UnaryClientInterceptor
	streamInterceptors []grpc.StreamClientInterceptor
	dialOptions        []grpc.DialOption

	// TLS
	tlsConfig *tls.Config

	// Resilience
	retryMaxAttempts int
	retryBackoff     time.Duration
	circuitBreaker   circuitbreaker.CircuitBreaker

	// Observability
	tracerName       string
	metricsCollector *metrics.PrometheusCollector
	enableLogging    bool

	// Load Balancing
	balancerPolicy string

	// Timeout
	timeout time.Duration

	// Keepalive
	keepaliveTime    time.Duration
	keepaliveTimeout time.Duration
}

// defaultOptions 返回默认配置.
func defaultOptions() *options {
	return &options{
		name:             "gRPC-Client",
		keepaliveTime:    60 * time.Second,
		keepaliveTimeout: 20 * time.Second,
	}
}

// WithName 设置客户端名称（用于日志）.
func WithName(name string) Option {
	return func(o *options) {
		o.name = name
	}
}

// WithServiceName 设置目标服务名称（必需）.
func WithServiceName(name string) Option {
	return func(o *options) {
		o.serviceName = name
	}
}

// WithDiscovery 设置服务发现实例（必需）.
func WithDiscovery(d discovery.Discovery) Option {
	return func(o *options) {
		o.discovery = d
	}
}

// WithLogger 设置日志实例.
func WithLogger(l logger.Logger) Option {
	return func(o *options) {
		o.logger = l
	}
}

// WithInterceptors 添加自定义一元拦截器.
func WithInterceptors(interceptors ...grpc.UnaryClientInterceptor) Option {
	return func(o *options) {
		o.interceptors = append(o.interceptors, interceptors...)
	}
}

// WithStreamInterceptors 添加自定义流拦截器.
func WithStreamInterceptors(interceptors ...grpc.StreamClientInterceptor) Option {
	return func(o *options) {
		o.streamInterceptors = append(o.streamInterceptors, interceptors...)
	}
}

// WithDialOptions 添加额外的 dial 选项.
func WithDialOptions(opts ...grpc.DialOption) Option {
	return func(o *options) {
		o.dialOptions = append(o.dialOptions, opts...)
	}
}

// WithTLS 设置 TLS 配置.
func WithTLS(cfg *tls.Config) Option {
	return func(o *options) {
		o.tlsConfig = cfg
	}
}

// WithRetry 设置重试配置.
func WithRetry(maxAttempts int, backoff time.Duration) Option {
	return func(o *options) {
		o.retryMaxAttempts = maxAttempts
		o.retryBackoff = backoff
	}
}

// WithCircuitBreaker 设置熔断器.
func WithCircuitBreaker(cb circuitbreaker.CircuitBreaker) Option {
	return func(o *options) {
		o.circuitBreaker = cb
	}
}

// WithTracing 设置链路追踪 tracer 名称.
func WithTracing(serviceName string) Option {
	return func(o *options) {
		o.tracerName = serviceName
	}
}

// WithMetrics 设置指标收集器.
func WithMetrics(collector *metrics.PrometheusCollector) Option {
	return func(o *options) {
		o.metricsCollector = collector
	}
}

// WithLogging 启用日志拦截器.
func WithLogging() Option {
	return func(o *options) {
		o.enableLogging = true
	}
}

// WithBalancer 设置负载均衡策略，支持 "round_robin" 或 "pick_first".
func WithBalancer(policy string) Option {
	return func(o *options) {
		o.balancerPolicy = policy
	}
}

// WithTimeout 设置 dial 超时.
func WithTimeout(d time.Duration) Option {
	return func(o *options) {
		o.timeout = d
	}
}

// WithKeepalive 设置 keepalive 参数.
func WithKeepalive(t, timeout time.Duration) Option {
	return func(o *options) {
		o.keepaliveTime = t
		o.keepaliveTimeout = timeout
	}
}
