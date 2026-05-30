package gateway

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/Tsukikage7/servex/v2/auth"
	"github.com/Tsukikage7/servex/v2/auth/grpcx"
	"github.com/Tsukikage7/servex/v2/httpx/clientip"
	"github.com/Tsukikage7/servex/v2/middleware/cors"
	"github.com/Tsukikage7/servex/v2/middleware/logging"
	"github.com/Tsukikage7/servex/v2/middleware/ratelimit"
	"github.com/Tsukikage7/servex/v2/middleware/recovery"
	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/observability/metrics"
	"github.com/Tsukikage7/servex/v2/observability/tracing"
	"github.com/Tsukikage7/servex/v2/tenant"
	"github.com/Tsukikage7/servex/v2/transport"
	"github.com/Tsukikage7/servex/v2/transport/health"
	"github.com/Tsukikage7/servex/v2/transport/response"
)

// Option 配置选项.
type Option func(*options)

// RuntimeConfig 是 Gateway 运行时行为配置。
type RuntimeConfig = transport.RuntimeConfig

// ObservabilityConfig 聚合 Gateway 的观测相关选项.
type ObservabilityConfig struct {
	TracingService   string
	TracingSkipPaths []string
	Metrics          *metrics.PrometheusCollector
	Logging          bool
	LoggingSkipPaths []string
}

// SecurityConfig 聚合 Gateway 的安全与入口治理选项.
type SecurityConfig struct {
	Authenticator auth.Authenticator
	AuthOptions   []auth.Option
	CORS          bool
	CORSOptions   []cors.Option
	RateLimiter   ratelimit.Limiter
	ClientIP      bool
	ClientIPOpts  []clientip.Option
	Tenant        tenant.Resolver
	TenantOptions []tenant.Option
}

type options struct {
	name     string
	version  string
	services []Registrar

	// gRPC
	grpcAddr           string
	enableReflection   bool
	keepaliveTime      time.Duration
	keepaliveTimeout   time.Duration
	minPingInterval    time.Duration
	unaryInterceptors  []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor
	grpcServerOpts     []grpc.ServerOption

	// HTTP
	httpAddr         string
	httpReadTimeout  time.Duration
	httpWriteTimeout time.Duration
	httpIdleTimeout  time.Duration

	// Gateway
	dialOptions    []grpc.DialOption
	serveMuxOpts   []runtime.ServeMuxOption
	marshalOptions protojson.MarshalOptions

	// Health (内置)
	healthTimeout time.Duration
	healthOptions []health.Option

	// HTTP 中间件链按添加顺序从外到内包装
	httpMiddlewares []func(http.Handler) http.Handler

	// Trace
	tracerName       string   // 链路追踪服务名，为空则不启用
	tracingSkipPaths []string // 不产生 trace span 的 HTTP 路径

	// Response
	enableResponse bool // 是否启用统一响应格式

	// Recovery
	enableRecovery bool // 是否启用 panic 恢复

	// CORS仅 HTTP
	corsOpts   []cors.Option
	enableCORS bool

	// Logging
	enableLogging    bool
	loggingSkipPaths []string

	// Metrics
	metricsCollector *metrics.PrometheusCollector

	// RateLimit
	rateLimiter ratelimit.Limiter

	// ClientIP
	enableClientIP bool
	clientIPOpts   []clientip.Option

	// Tenant
	tenantResolver tenant.Resolver
	tenantOpts     []tenant.Option

	// HTTP TLS
	httpTLSConfig *tls.Config

	// gRPC Gateway 连接 TLSconnectGateway 回连 gRPC 时使用
	grpcTLSConfig *tls.Config

	// Auth
	authenticator      auth.Authenticator
	authOptions        []auth.Option
	discoveredPolicies auth.MethodPolicyMap

	logger logger.Logger
}

func defaultOptions() *options {
	return &options{
		name:             "Gateway",
		grpcAddr:         ":9090",
		httpAddr:         ":8080",
		enableReflection: true,
		keepaliveTime:    60 * time.Second,
		keepaliveTimeout: 20 * time.Second,
		minPingInterval:  20 * time.Second,
		httpReadTimeout:  30 * time.Second,
		httpWriteTimeout: 30 * time.Second,
		httpIdleTimeout:  120 * time.Second,
		healthTimeout:    5 * time.Second,
	}
}

// WithName 设置服务名称.
func WithName(name string) Option {
	return func(o *options) { o.name = name }
}

// WithVersion 设置服务版本，会注入到健康检查响应中.
func WithVersion(v string) Option {
	return func(o *options) { o.version = v }
}

// WithGRPCAddr 设置 gRPC 地址.
func WithGRPCAddr(addr string) Option {
	return func(o *options) { o.grpcAddr = addr }
}

// WithHTTPAddr 设置 HTTP 地址.
func WithHTTPAddr(addr string) Option {
	return func(o *options) { o.httpAddr = addr }
}

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option {
	return func(o *options) { o.logger = log }
}

// WithConfig 从配置结构体设置服务器选项.
// 仅设置非零值字段，零值字段将保持默认值.
func WithConfig(cfg transport.GatewayConfig) Option {
	return func(o *options) {
		if cfg.Name != "" {
			o.name = cfg.Name
		}
		if cfg.Version != "" {
			o.version = cfg.Version
		}
		if cfg.GRPC.Name != "" && cfg.Name == "" {
			o.name = cfg.GRPC.Name
		}
		if cfg.GRPC.Addr != "" {
			o.grpcAddr = cfg.GRPC.Addr
		}
		if cfg.GRPC.EnableReflection != nil {
			o.enableReflection = *cfg.GRPC.EnableReflection
		}
		if cfg.GRPC.KeepaliveTime > 0 {
			o.keepaliveTime = cfg.GRPC.KeepaliveTime
		}
		if cfg.GRPC.KeepaliveTimeout > 0 {
			o.keepaliveTimeout = cfg.GRPC.KeepaliveTimeout
		}
		if cfg.HTTP.Name != "" && cfg.Name == "" && cfg.GRPC.Name == "" {
			o.name = cfg.HTTP.Name
		}
		if cfg.HTTP.Addr != "" {
			o.httpAddr = cfg.HTTP.Addr
		}
		if cfg.HTTP.ReadTimeout > 0 {
			o.httpReadTimeout = cfg.HTTP.ReadTimeout
		}
		if cfg.HTTP.WriteTimeout > 0 {
			o.httpWriteTimeout = cfg.HTTP.WriteTimeout
		}
		if cfg.HTTP.IdleTimeout > 0 {
			o.httpIdleTimeout = cfg.HTTP.IdleTimeout
		}
		if cfg.HealthTimeout > 0 {
			o.healthTimeout = cfg.HealthTimeout
		}
		if cfg.Runtime.Recovery || cfg.Runtime.Response {
			WithRuntime(cfg.Runtime)(o)
		}
		if cfg.Logging.Enabled {
			o.enableLogging = true
			o.loggingSkipPaths = cfg.Logging.SkipPaths
		}
		if cfg.Tracing.Enabled {
			service := cfg.Tracing.Service
			if service == "" {
				service = cfg.Name
			}
			if service != "" {
				o.tracerName = service
				o.tracingSkipPaths = cfg.Tracing.SkipPaths
				o.unaryInterceptors = append(
					[]grpc.UnaryServerInterceptor{tracing.UnaryServerInterceptor(service)},
					o.unaryInterceptors...,
				)
				o.streamInterceptors = append(
					[]grpc.StreamServerInterceptor{tracing.StreamServerInterceptor(service)},
					o.streamInterceptors...,
				)
			}
		}
		if cfg.CORS.Enabled {
			o.enableCORS = true
			o.corsOpts = cors.MiddlewareOptions(&cors.Options{
				AllowOrigins:     cfg.CORS.AllowOrigins,
				AllowMethods:     cfg.CORS.AllowMethods,
				AllowHeaders:     cfg.CORS.AllowHeaders,
				ExposeHeaders:    cfg.CORS.ExposeHeaders,
				AllowCredentials: cfg.CORS.AllowCredentials,
				MaxAge:           cfg.CORS.MaxAge,
			}, false)
		}
	}
}

// WithReflection 启用/禁用 gRPC 反射.
func WithReflection(enabled bool) Option {
	return func(o *options) { o.enableReflection = enabled }
}

// WithKeepalive 设置 gRPC keepalive 参数.
func WithKeepalive(t, timeout time.Duration) Option {
	return func(o *options) {
		o.keepaliveTime = t
		o.keepaliveTimeout = timeout
	}
}

// WithUnaryInterceptor 添加 gRPC 一元拦截器.
func WithUnaryInterceptor(interceptors ...grpc.UnaryServerInterceptor) Option {
	return func(o *options) {
		o.unaryInterceptors = append(o.unaryInterceptors, interceptors...)
	}
}

// WithStreamInterceptor 添加 gRPC 流拦截器.
func WithStreamInterceptor(interceptors ...grpc.StreamServerInterceptor) Option {
	return func(o *options) {
		o.streamInterceptors = append(o.streamInterceptors, interceptors...)
	}
}

// WithGRPCServerOption 添加 gRPC 服务器选项.
func WithGRPCServerOption(opts ...grpc.ServerOption) Option {
	return func(o *options) {
		o.grpcServerOpts = append(o.grpcServerOpts, opts...)
	}
}

// WithHTTPTimeout 设置 HTTP 超时.
func WithHTTPTimeout(read, write, idle time.Duration) Option {
	return func(o *options) {
		o.httpReadTimeout = read
		o.httpWriteTimeout = write
		o.httpIdleTimeout = idle
	}
}

// WithDialOptions 添加 gRPC 拨号选项.
func WithDialOptions(opts ...grpc.DialOption) Option {
	return func(o *options) {
		o.dialOptions = append(o.dialOptions, opts...)
	}
}

// WithServeMuxOptions 添加 ServeMux 选项.
func WithServeMuxOptions(opts ...runtime.ServeMuxOption) Option {
	return func(o *options) {
		o.serveMuxOpts = append(o.serveMuxOpts, opts...)
	}
}

// WithMarshalOptions 设置 JSON 序列化选项.
func WithMarshalOptions(opts protojson.MarshalOptions) Option {
	return func(o *options) { o.marshalOptions = opts }
}

// WithHealthTimeout 设置健康检查超时时间.
func WithHealthTimeout(d time.Duration) Option {
	return func(o *options) { o.healthTimeout = d }
}

// WithHealthOptions 添加健康检查选项.
//
// 例如添加就绪检查器:
//
//	WithHealthOptions(
//	    health.WithReadinessChecker(health.NewDBChecker("db", db)),
//	)
func WithHealthOptions(opts ...health.Option) Option {
	return func(o *options) {
		o.healthOptions = append(o.healthOptions, opts...)
	}
}

// WithObservability 启用观测相关能力.
func WithObservability(cfg ObservabilityConfig) Option {
	return func(o *options) {
		if cfg.TracingService != "" {
			o.tracerName = cfg.TracingService
			o.tracingSkipPaths = cfg.TracingSkipPaths
			o.unaryInterceptors = append(
				[]grpc.UnaryServerInterceptor{tracing.UnaryServerInterceptor(cfg.TracingService)},
				o.unaryInterceptors...,
			)
			o.streamInterceptors = append(
				[]grpc.StreamServerInterceptor{tracing.StreamServerInterceptor(cfg.TracingService)},
				o.streamInterceptors...,
			)
		}
		if cfg.Metrics != nil {
			o.metricsCollector = cfg.Metrics
		}
		if cfg.Logging {
			o.enableLogging = true
			o.loggingSkipPaths = cfg.LoggingSkipPaths
		}
	}
}

// WithRuntime 启用 Gateway 运行时行为。
func WithRuntime(cfg RuntimeConfig) Option {
	return func(o *options) {
		if cfg.Response {
			o.enableResponse = true
			o.unaryInterceptors = append(o.unaryInterceptors, response.UnaryServerInterceptor())
		}
		if cfg.Recovery {
			o.enableRecovery = true
		}
	}
}

// WithSecurity 启用安全与入口治理能力.
func WithSecurity(cfg SecurityConfig) Option {
	return func(o *options) {
		if cfg.Authenticator != nil {
			o.authenticator = cfg.Authenticator
			o.authOptions = cfg.AuthOptions
		}
		if cfg.CORS {
			o.enableCORS = true
			o.corsOpts = cfg.CORSOptions
		}
		if cfg.RateLimiter != nil {
			o.rateLimiter = cfg.RateLimiter
		}
		if cfg.ClientIP {
			o.enableClientIP = true
			o.clientIPOpts = cfg.ClientIPOpts
		}
		if cfg.Tenant != nil {
			o.tenantResolver = cfg.Tenant
			o.tenantOpts = cfg.TenantOptions
		}
	}
}

// WithHTTPTLS 启用 HTTP 端 TLS.
//
// 传入 *tls.Config 后，HTTP 服务器启动时将使用 ListenAndServeTLS.
// gRPC 端的 TLS 需要通过 gRPC DialOption 或 ServerOption 单独配置.
//
// 示例:
//
//	tlsCfg, _ := tlsx.NewServerTLSConfig(&tlsx.Config{
//	    CertFile: "server.crt",
//	    KeyFile:  "server.key",
//	})
//	gateway.WithHTTPTLS(tlsCfg)
func WithHTTPTLS(cfg *tls.Config) Option {
	return func(o *options) {
		o.httpTLSConfig = cfg
	}
}

// WithGRPCTLS 启用 gRPC Gateway 回连 TLS.
//
// 传入 *tls.Config 后，connectGateway 将使用 TLS 凭证连接 gRPC 服务，
// 而非默认的 insecure 连接.
func WithGRPCTLS(cfg *tls.Config) Option {
	return func(o *options) {
		o.grpcTLSConfig = cfg
	}
}

// WithHTTPMiddleware 追加自定义 HTTP 中间件到 gateway 的 HTTP 处理链.
//
// 多次调用会按调用顺序 append；执行顺序与内置中间件一致：后 append 的先执行。
// 典型用法：请求体大小限制、按 IP/Token 限流、自定义签名校验等.
//
// 示例:
//
//	import "github.com/Tsukikage7/servex/v2/middleware/bodylimit"
//
//	gateway.WithHTTPMiddleware(bodylimit.HTTPMiddleware(1 << 20)) // 1MB
func WithHTTPMiddleware(mws ...func(http.Handler) http.Handler) Option {
	return func(o *options) {
		o.httpMiddlewares = append(o.httpMiddlewares, mws...)
	}
}

// applyNewInterceptors 按照正确的顺序应用新增的 gRPC 拦截器.
//
// 拦截器按追加顺序执行，因此先添加的先执行:
// Logging → Metrics → RateLimit → ClientIP → Tenant
// Recovery 和 Auth 由各自的 apply 函数处理，Tracing 由 WithObservability 添加
func applyNewInterceptors(o *options) {
	// Logging
	if o.enableLogging && o.logger != nil {
		loggingOpts := []logging.Option{
			logging.WithLogger(o.logger),
		}
		if len(o.loggingSkipPaths) > 0 {
			loggingOpts = append(loggingOpts, logging.WithSkipPaths(o.loggingSkipPaths...))
		}
		o.unaryInterceptors = append(o.unaryInterceptors, logging.UnaryServerInterceptor(loggingOpts...))
		o.streamInterceptors = append(o.streamInterceptors, logging.StreamServerInterceptor(loggingOpts...))
	}

	// Metrics
	if o.metricsCollector != nil {
		o.unaryInterceptors = append(o.unaryInterceptors, metrics.UnaryServerInterceptor(o.metricsCollector))
		o.streamInterceptors = append(o.streamInterceptors, metrics.StreamServerInterceptor(o.metricsCollector))
	}

	// RateLimit
	if o.rateLimiter != nil {
		o.unaryInterceptors = append(o.unaryInterceptors, ratelimit.UnaryServerInterceptor(o.rateLimiter))
		o.streamInterceptors = append(o.streamInterceptors, ratelimit.StreamServerInterceptor(o.rateLimiter))
	}

	// ClientIP
	if o.enableClientIP {
		o.unaryInterceptors = append(o.unaryInterceptors, clientip.UnaryServerInterceptor(o.clientIPOpts...))
		o.streamInterceptors = append(o.streamInterceptors, clientip.StreamServerInterceptor(o.clientIPOpts...))
	}

	// Tenant
	if o.tenantResolver != nil {
		tenantOpts := o.tenantOpts
		if o.logger != nil {
			tenantOpts = append(tenantOpts, tenant.WithLogger(o.logger))
		}
		o.unaryInterceptors = append(o.unaryInterceptors, tenant.UnaryServerInterceptor(o.tenantResolver, tenantOpts...))
		o.streamInterceptors = append(o.streamInterceptors, tenant.StreamServerInterceptor(o.tenantResolver, tenantOpts...))
	}
}

// applyRecoveryInterceptors 应用 recovery 拦截器到拦截器链最前面.
func applyRecoveryInterceptors(o *options) {
	if !o.enableRecovery || o.logger == nil {
		return
	}
	o.unaryInterceptors = append(
		[]grpc.UnaryServerInterceptor{recovery.UnaryServerInterceptor(recovery.WithLogger(o.logger))},
		o.unaryInterceptors...,
	)
	o.streamInterceptors = append(
		[]grpc.StreamServerInterceptor{recovery.StreamServerInterceptor(recovery.WithLogger(o.logger))},
		o.streamInterceptors...,
	)
}

// applyAuthInterceptors 应用 auth 拦截器.
func applyAuthInterceptors(o *options) {
	if o.authenticator == nil {
		return
	}

	o.discoveredPolicies = make(auth.MethodPolicyMap)

	// 构建 proto public skipper
	skipper := buildProtoPolicySkipper(o)

	// 合并选项
	authOpts := append([]auth.Option{}, o.authOptions...)
	if skipper != nil {
		authOpts = append(authOpts, auth.WithSkipper(skipper))
	}
	if o.logger != nil {
		authOpts = append(authOpts, auth.WithLogger(o.logger))
	}
	if o.discoveredPolicies != nil {
		authOpts = append(authOpts, auth.WithPolicyProvider(o.discoveredPolicies))
	}

	// 添加到拦截器链在 recovery 之后
	o.unaryInterceptors = append(
		o.unaryInterceptors,
		grpcx.UnaryServerInterceptor(o.authenticator, authOpts...),
	)
	o.streamInterceptors = append(
		o.streamInterceptors,
		grpcx.StreamServerInterceptor(o.authenticator, authOpts...),
	)
}

// buildProtoPolicySkipper 构建 proto public 跳过器.
func buildProtoPolicySkipper(o *options) auth.Skipper {
	if o.discoveredPolicies == nil {
		return nil
	}

	return func(ctx context.Context, _ any) bool {
		method, ok := grpc.Method(ctx)
		if !ok {
			return false
		}

		policy, ok := o.discoveredPolicies[method]
		if ok && policy != nil && policy.Public {
			return true
		}

		return false
	}
}
