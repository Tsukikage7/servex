package grpcserver

import (
	"context"
	"crypto/tls"
	"time"

	"google.golang.org/grpc"

	"github.com/Tsukikage7/servex/auth"
	"github.com/Tsukikage7/servex/httpx/clientip"
	"github.com/Tsukikage7/servex/middleware/ratelimit"
	"github.com/Tsukikage7/servex/observability/logger"
	"github.com/Tsukikage7/servex/observability/metrics"
	"github.com/Tsukikage7/servex/observability/tracing"
	"github.com/Tsukikage7/servex/tenant"
	"github.com/Tsukikage7/servex/transport"
	"github.com/Tsukikage7/servex/transport/health"
)

// Option 配置选项函数.
type Option func(*options)

// options 服务器配置.
type options struct {
	name               string
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
	tracerName         string // 链路追踪服务名，为空则不启用
	enableRecovery     bool   // 是否启用 panic 恢复
	enableLogging      bool   // 是否启用请求日志
	loggingSkipPaths   []string

	// Auth
	authenticator       auth.Authenticator
	authOptions         []auth.Option
	publicMethods       []string        // 公开方法（无需认证）
	enableAutoDiscovery bool            // 启用 proto option 自动发现
	discoveredMethods   map[string]bool // 自动发现的公开方法（延迟填充）

	// Tenant
	tenantResolver tenant.Resolver
	tenantOptions  []tenant.Option

	// ClientIP
	enableClientIP  bool
	clientIPOptions []clientip.Option

	// Metrics
	metricsCollector *metrics.PrometheusCollector

	// RateLimit
	rateLimiter ratelimit.Limiter

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

// WithLogger 设置日志记录器（必需）.
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
		// EnableReflection 是 bool 类型，需要特殊处理
		// 由于无法区分 false 和零值，这里只在配置中显式设置时才应用
		o.enableReflection = cfg.EnableReflection
		if cfg.KeepaliveTime > 0 {
			o.keepaliveTime = cfg.KeepaliveTime
		}
		if cfg.KeepaliveTimeout > 0 {
			o.keepaliveTimeout = cfg.KeepaliveTimeout
		}
		if len(cfg.PublicMethods) > 0 {
			o.publicMethods = cfg.PublicMethods
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

// WithTrace 启用链路追踪.
//
// 注意: 需要先调用 tracing.NewTracer() 初始化全局 TracerProvider.
func WithTrace(serviceName string) Option {
	return func(o *options) {
		o.tracerName = serviceName
		// 将 trace 拦截器添加到拦截器链最前面
		o.unaryInterceptors = append(
			[]grpc.UnaryServerInterceptor{tracing.UnaryServerInterceptor(serviceName)},
			o.unaryInterceptors...,
		)
		o.streamInterceptors = append(
			[]grpc.StreamServerInterceptor{tracing.StreamServerInterceptor(serviceName)},
			o.streamInterceptors...,
		)
	}
}

// WithRecovery 启用 panic 恢复.
//
// 启用后，handler 中的 panic 会被捕获并记录，返回 codes.Internal 错误.
// 注意: recovery 拦截器会添加到拦截器链最前面，确保能捕获所有内层 panic.
func WithRecovery() Option {
	return func(o *options) {
		o.enableRecovery = true
	}
}

// WithLogging 启用 gRPC 请求访问日志.
//
// 日志拦截器位于 recovery 之外（最外层），可记录每个 RPC 的方法、状态码和耗时.
// 可通过 skipPaths 跳过不需要记录的方法名（如健康检查的 FullMethod）.
func WithLogging(skipPaths ...string) Option {
	return func(o *options) {
		o.enableLogging = true
		o.loggingSkipPaths = skipPaths
	}
}

// WithAuth 启用认证.
//
// 示例:
//
//	jwtService := jwt.NewJWT(jwt.WithSecretKey("secret"))
//	authenticator := jwt.NewAuthenticator(jwtService)
//
//	server := grpcserver.New(
//	    grpcserver.WithAuth(authenticator),
//	    grpcserver.WithPublicMethods(
//	        "/api.user.v1.AuthService/Login",
//	        "/api.user.v1.AuthService/Register",
//	    ),
//	)
func WithAuth(authenticator auth.Authenticator, opts ...auth.Option) Option {
	return func(o *options) {
		o.authenticator = authenticator
		o.authOptions = opts
	}
}

// WithPublicMethods 设置公开方法（无需认证）.
//
// 方法名格式为 gRPC 完整方法名，如:
//   - "/api.user.v1.AuthService/Login"
//   - "/api.user.v1.AuthService/Register"
//
// 也支持服务级别的通配:
//   - "/api.user.v1.AuthService/*" (该服务下所有方法)
func WithPublicMethods(methods ...string) Option {
	return func(o *options) {
		o.publicMethods = append(o.publicMethods, methods...)
	}
}

// WithAutoDiscovery 启用 proto option 自动发现.
//
// 启用后，服务器会在启动时自动扫描注册的 gRPC 服务，
// 从 proto 定义中发现标记为 public 的方法，无需手动配置 WithPublicMethods.
//
// 在 proto 中标记公开方法:
//
//	import "github.com/Tsukikage7/servex/auth/proto/auth.proto";
//
//	service AuthService {
//	  rpc Login(LoginRequest) returns (LoginResponse) {
//	    option (microservice.kit.auth.method) = {
//	      public: true
//	    };
//	  }
//	}
//
// 注意: 自动发现会与手动配置的 WithPublicMethods 合并.
func WithAutoDiscovery() Option {
	return func(o *options) {
		o.enableAutoDiscovery = true
	}
}

// applyAuthInterceptors 应用 auth 拦截器.
func applyAuthInterceptors(o *options) {
	if o.authenticator == nil {
		return
	}

	// 如果启用自动发现，初始化 map
	if o.enableAutoDiscovery {
		o.discoveredMethods = make(map[string]bool)
	}

	// 构建 skipper（支持手动配置 + 自动发现）
	skipper := buildCombinedSkipper(o)

	// 合并选项
	authOpts := append([]auth.Option{}, o.authOptions...)
	if skipper != nil {
		authOpts = append(authOpts, auth.WithSkipper(skipper))
	}
	if o.logger != nil {
		authOpts = append(authOpts, auth.WithLogger(o.logger))
	}

	// 添加到拦截器链
	o.unaryInterceptors = append(
		o.unaryInterceptors,
		auth.UnaryServerInterceptor(o.authenticator, authOpts...),
	)
	o.streamInterceptors = append(
		o.streamInterceptors,
		auth.StreamServerInterceptor(o.authenticator, authOpts...),
	)
}

// buildCombinedSkipper 构建组合跳过器（手动配置 + 自动发现）.
func buildCombinedSkipper(o *options) auth.Skipper {
	// 如果没有任何配置，返回 nil
	if len(o.publicMethods) == 0 && !o.enableAutoDiscovery {
		return nil
	}

	skip := transport.BuildMethodSkipper(o.publicMethods)

	return func(ctx context.Context, _ any) bool {
		method, ok := grpc.Method(ctx)
		if !ok {
			return false
		}

		// 1. 检查手动配置的方法（精确匹配 + 前缀匹配）
		if skip(method) {
			return true
		}

		// 2. 检查自动发现的方法（延迟填充）
		if o.discoveredMethods != nil && o.discoveredMethods[method] {
			return true
		}

		return false
	}
}

// buildMethodSkipper 构建方法跳过器.
func buildMethodSkipper(publicMethods []string) auth.Skipper {
	skip := transport.BuildMethodSkipper(publicMethods)
	return func(ctx context.Context, _ any) bool {
		method, ok := grpc.Method(ctx)
		if !ok {
			return false
		}
		return skip(method)
	}
}

// WithTenant 启用多租户解析.
//
// 租户拦截器位于 auth 之后（更靠近 handler），确保先完成认证再解析租户.
//
// 示例:
//
//	server := grpcserver.New(
//	    grpcserver.WithAuth(authenticator),
//	    grpcserver.WithTenant(resolver,
//	        tenant.WithTokenExtractor(tenant.MetadataTokenExtractor("x-tenant-id")),
//	    ),
//	)
func WithTenant(resolver tenant.Resolver, opts ...tenant.Option) Option {
	return func(o *options) {
		o.tenantResolver = resolver
		o.tenantOptions = opts
	}
}

// WithClientIP 启用客户端 IP 提取.
//
// 启用后，可以通过 clientip.GetIP(ctx) 获取客户端真实 IP.
//
// 示例:
//
//	server := grpcserver.New(
//	    grpcserver.WithClientIP(),  // 默认配置
//	)
//
//	// 或指定可信代理
//	server := grpcserver.New(
//	    grpcserver.WithClientIP(
//	        clientip.WithTrustedProxies("10.0.0.0/8"),
//	    ),
//	)
func WithClientIP(opts ...clientip.Option) Option {
	return func(o *options) {
		o.enableClientIP = true
		o.clientIPOptions = opts
	}
}

// WithMetrics 启用 Prometheus 指标收集.
//
// 启用后，gRPC 端记录方法名、状态码和耗时.
//
// 示例:
//
//	collector, _ := metrics.New(metricsCfg)
//	grpcserver.New(grpcserver.WithMetrics(collector))
func WithMetrics(collector *metrics.PrometheusCollector) Option {
	return func(o *options) {
		o.metricsCollector = collector
	}
}

// WithRateLimit 启用限流.
//
// 当请求被限流时，gRPC 端返回 ResourceExhausted 错误.
//
// 示例:
//
//	limiter := ratelimit.NewTokenBucket(100, 200)
//	grpcserver.New(grpcserver.WithRateLimit(limiter))
func WithRateLimit(limiter ratelimit.Limiter) Option {
	return func(o *options) {
		o.rateLimiter = limiter
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
