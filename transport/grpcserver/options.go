package grpcserver

import (
	"crypto/tls"
	"time"

	"google.golang.org/grpc"

	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/transport"
	"github.com/Tsukikage7/servex/v2/transport/health"
)

// Option 配置选项函数.
type Option func(*options)

// options 服务器配置.
type options struct {
	name               string
	version            string
	addr               string
	enableReflection   bool
	keepaliveTime      time.Duration
	keepaliveTimeout   time.Duration
	minPingInterval    time.Duration
	logger             logger.Logger
	services           []Registrar
	unaryInterceptors  []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor
	serverOptions      []grpc.ServerOption
	healthTimeout      time.Duration
	healthOptions      []health.Option

	// TLS
	tlsConfig *tls.Config
}

// defaultOptions 返回默认配置.
func defaultOptions() *options {
	return &options{
		name:             "gRPC",
		addr:             ":9090",
		enableReflection: true,
		keepaliveTime:    60 * time.Second,
		keepaliveTimeout: 20 * time.Second,
		minPingInterval:  20 * time.Second,
		healthTimeout:    5 * time.Second,
	}
}

// WithName 设置服务器名称.
func WithName(name string) Option {
	return func(o *options) {
		o.name = name
	}
}

// WithVersion 设置服务版本，会注入到健康检查响应中.
func WithVersion(v string) Option {
	return func(o *options) {
		o.version = v
	}
}

// WithAddr 设置监听地址.
func WithAddr(addr string) Option {
	return func(o *options) {
		o.addr = addr
	}
}

// WithReflection 设置是否启用反射.
func WithReflection(enabled bool) Option {
	return func(o *options) {
		o.enableReflection = enabled
	}
}

// WithKeepalive 设置 Keepalive 参数.
func WithKeepalive(time, timeout time.Duration) Option {
	return func(o *options) {
		o.keepaliveTime = time
		o.keepaliveTimeout = timeout
	}
}

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option {
	return func(o *options) {
		o.logger = log
	}
}

// WithUnaryInterceptor 添加一元拦截器.
func WithUnaryInterceptor(interceptors ...grpc.UnaryServerInterceptor) Option {
	return func(o *options) {
		o.unaryInterceptors = append(o.unaryInterceptors, interceptors...)
	}
}

// WithStreamInterceptor 添加流拦截器.
func WithStreamInterceptor(interceptors ...grpc.StreamServerInterceptor) Option {
	return func(o *options) {
		o.streamInterceptors = append(o.streamInterceptors, interceptors...)
	}
}

// WithServerOption 添加自定义 gRPC 服务器选项.
func WithServerOption(opts ...grpc.ServerOption) Option {
	return func(o *options) {
		o.serverOptions = append(o.serverOptions, opts...)
	}
}

// WithConfig 从配置结构体设置服务器选项.
// 仅设置非零值字段，零值字段将保持默认值.
func WithConfig(cfg transport.GRPCConfig) Option {
	return func(o *options) {
		if cfg.Name != "" {
			o.name = cfg.Name
		}
		if cfg.Addr != "" {
			o.addr = cfg.Addr
		}
		// EnableReflection 使用 *bool 区分零值和未设置
		if cfg.EnableReflection != nil {
			o.enableReflection = *cfg.EnableReflection
		}
		if cfg.KeepaliveTime > 0 {
			o.keepaliveTime = cfg.KeepaliveTime
		}
		if cfg.KeepaliveTimeout > 0 {
			o.keepaliveTimeout = cfg.KeepaliveTimeout
		}
	}
}

// WithHealthTimeout 设置健康检查超时时间.
func WithHealthTimeout(d time.Duration) Option {
	return func(o *options) {
		o.healthTimeout = d
	}
}

// WithHealthOptions 添加健康检查选项.
func WithHealthOptions(opts ...health.Option) Option {
	return func(o *options) {
		o.healthOptions = append(o.healthOptions, opts...)
	}
}

// WithReadinessChecker 添加就绪检查器（便捷方法）.
func WithReadinessChecker(checkers ...health.Checker) Option {
	return func(o *options) {
		o.healthOptions = append(o.healthOptions, health.WithReadinessChecker(checkers...))
	}
}

// WithLivenessChecker 添加存活检查器（便捷方法）.
func WithLivenessChecker(checkers ...health.Checker) Option {
	return func(o *options) {
		o.healthOptions = append(o.healthOptions, health.WithLivenessChecker(checkers...))
	}
}

// WithTLS 启用 TLS.
//
// 传入 *tls.Config 后，服务器将通过 grpc.Creds 添加 TLS 传输凭据.
// 可配合 transport/tls (tlsx) 包生成配置：
//
//	tlsCfg, _ := tlsx.NewServerTLSConfig(&tlsx.Config{
//	    CertFile: "server.crt",
//	    KeyFile:  "server.key",
//	})
//	grpcserver.New(grpcserver.WithTLS(tlsCfg))
func WithTLS(cfg *tls.Config) Option {
	return func(o *options) {
		o.tlsConfig = cfg
	}
}
