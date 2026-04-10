package gateway

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/Tsukikage7/servex/auth"
	"github.com/Tsukikage7/servex/httpx/clientip"
	"github.com/Tsukikage7/servex/middleware/cors"
	"github.com/Tsukikage7/servex/middleware/logging"
	"github.com/Tsukikage7/servex/middleware/ratelimit"
	"github.com/Tsukikage7/servex/middleware/recovery"
	"github.com/Tsukikage7/servex/observability/logger"
	"github.com/Tsukikage7/servex/observability/metrics"
	"github.com/Tsukikage7/servex/observability/tracing"
	"github.com/Tsukikage7/servex/tenant"
	"github.com/Tsukikage7/servex/transport"
	"github.com/Tsukikage7/servex/transport/health"
	"github.com/Tsukikage7/servex/transport/response"
)

// Option 配置选项.
type Option func(*options)

type options struct {
	name     string
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

	// HTTP 中间件链（按添加顺序从外到内包装）
	httpMiddlewares []func(http.Handler) http.Handler

	// Trace
	tracerName string // 链路追踪服务名，为空则不启用

	// Response
	enableResponse bool // 是否启用统一响应格式

	// Recovery
	enableRecovery bool // 是否启用 panic 恢复

	// CORS（仅 HTTP）
	corsOpts   []cors.Option
	enableCORS bool

	// RequestID（已由 trace 中间件统一处理，保留开关兼容）
	enableRequestID bool

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

	// gRPC Gateway 连接 TLS（connectGateway 回连 gRPC 时使用）
	grpcTLSConfig *tls.Config

	// Auth
	authenticator       auth.Authenticator
	authOptions         []auth.Option
	publicMethods       []string        // 公开方法（无需认证）
	enableAutoDiscovery bool            // 启用 proto option 自动发现
	discoveredMethods   map[string]bool // 自动发现的公开方法（延迟填充）

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
		if cfg.GRPCAddr != "" {
			o.grpcAddr = cfg.GRPCAddr
		}
		if cfg.HTTPAddr != "" {
			o.httpAddr = cfg.HTTPAddr
		}
		if cfg.KeepaliveTime > 0 {
			o.keepaliveTime = cfg.KeepaliveTime
		}
		if len(cfg.PublicMethods) > 0 {
			o.publicMethods = cfg.PublicMethods
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

// WithTrace 启用链路追踪（gRPC + HTTP 双端）.
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

// WithResponse 启用统一响应格式（gRPC + HTTP 双端）.
//
// 启用后:
//   - HTTP 错误响应将使用统一的 JSON 格式: {"code": xxx, "message": "xxx"}
//   - gRPC 错误将自动映射到正确的状态码
//   - 内部错误（5xxxx）的详细信息将被隐藏
func WithResponse() Option {
	return func(o *options) {
		o.enableResponse = true
		// 将 response 拦截器添加到拦截器链末尾（在业务逻辑之后处理错误）
		o.unaryInterceptors = append(o.unaryInterceptors, response.UnaryServerInterceptor())
	}
}

// WithRecovery 启用 panic 恢复（gRPC + HTTP 双端）.
//
// 启用后，handler 中的 panic 会被捕获并记录:
//   - gRPC: 返回 codes.Internal 错误
//   - HTTP: 返回 500 状态码
func WithRecovery() Option {
	return func(o *options) {
		o.enableRecovery = true
	}
}

// WithAuth 启用认证（gRPC + HTTP 双端）.
//
// 由于 gRPC-Gateway 会将 HTTP 请求转换为 gRPC 调用，
// 只需在 gRPC 层添加认证拦截器即可同时保护两种协议。
//
// 示例:
//
//	jwtSrv := jwt.NewJWT(jwt.WithSecretKey("secret"))
//	authenticator := jwt.NewAuthenticator(jwtSrv)
//
//	server := gateway.New(
//	    gateway.WithAuth(authenticator),
//	    gateway.WithPublicMethods(
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
//   - "/api.product.v1.ProductService/List"
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
// 也可以标记整个服务为公开:
//
//	service PublicService {
//	  option (microservice.kit.auth.service) = {
//	    public: true
//	  };
//	}
//
// 注意: 自动发现会与手动配置的 WithPublicMethods 合并.
func WithAutoDiscovery() Option {
	return func(o *options) {
		o.enableAutoDiscovery = true
	}
}

// WithCORS 启用 CORS 中间件（仅 HTTP 端）.
//
// CORS 处理仅在 HTTP 层进行，gRPC 端不需要 CORS 支持.
//
// 示例:
//
//	gateway.WithCORS(
//	    cors.WithAllowOrigins("https://example.com"),
//	    cors.WithAllowCredentials(true),
//	)
func WithCORS(opts ...cors.Option) Option {
	return func(o *options) {
		o.enableCORS = true
		o.corsOpts = opts
	}
}

// WithRateLimit 启用限流（gRPC + HTTP 双端）.
//
// HTTP 端使用 ratelimit.HTTPMiddleware 返回 429 状态码；
// gRPC 端使用一元和流拦截器返回 ResourceExhausted 错误.
//
// 示例:
//
//	limiter := ratelimit.NewTokenBucket(100, 200)
//	gateway.WithRateLimit(limiter)
func WithRateLimit(limiter ratelimit.Limiter) Option {
	return func(o *options) {
		o.rateLimiter = limiter
	}
}

// WithMetrics 启用指标采集（gRPC + HTTP 双端）.
//
// HTTP 端记录请求方法、路径、状态码和耗时；
// gRPC 端记录方法名、状态码和耗时.
//
// 示例:
//
//	collector, _ := metrics.New(metricsCfg)
//	gateway.WithMetrics(collector)
func WithMetrics(collector *metrics.PrometheusCollector) Option {
	return func(o *options) {
		o.metricsCollector = collector
	}
}

// WithLogging 启用请求日志（gRPC + HTTP 双端）.
//
// HTTP 端记录方法、路径、状态码和耗时；
// gRPC 端记录方法名、状态码和耗时.
// 可通过 skipMethods 跳过不需要记录的 gRPC 方法或 HTTP 路径.
//
// 示例:
//
//	gateway.WithLogging("/grpc.health.v1.Health/Check")
func WithLogging(skipMethods ...string) Option {
	return func(o *options) {
		o.enableLogging = true
		o.loggingSkipPaths = skipMethods
	}
}

// WithTenant 启用多租户解析（gRPC + HTTP 双端）.
//
// HTTP 端和 gRPC 端分别使用对应的租户中间件和拦截器.
//
// 示例:
//
//	gateway.WithTenant(resolver,
//	    tenant.WithTokenExtractor(tenant.HeaderTokenExtractor("X-Tenant-ID")),
//	)
func WithTenant(resolver tenant.Resolver, opts ...tenant.Option) Option {
	return func(o *options) {
		o.tenantResolver = resolver
		o.tenantOpts = opts
	}
}

// WithClientIP 启用客户端 IP 提取（gRPC + HTTP 双端）.
//
// HTTP 端从 X-Forwarded-For / X-Real-IP / RemoteAddr 提取；
// gRPC 端从 metadata 和 peer 地址提取.
//
// 示例:
//
//	gateway.WithClientIP(clientip.WithTrustPrivateProxies())
func WithClientIP(opts ...clientip.Option) Option {
	return func(o *options) {
		o.enableClientIP = true
		o.clientIPOpts = opts
	}
}

// WithRequestID 启用 Request ID（gRPC + HTTP 双端）.
//
// Deprecated: Request ID 已由 trace 中间件统一处理，此选项保留兼容但不再生效.
//
// 示例:
//
//	gateway.WithRequestID()
func WithRequestID() Option {
	return func(o *options) {
		o.enableRequestID = true
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

// applyNewInterceptors 按照正确的顺序应用新增的 gRPC 拦截器.
//
// 拦截器按追加顺序执行，因此先添加的先执行:
// Logging → Metrics → RateLimit → ClientIP → Tenant
// （Recovery 和 Auth 由各自的 apply 函数处理，Tracing 在 WithTrace 中直接添加）
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

	// 添加到拦截器链（在 recovery 之后）
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
