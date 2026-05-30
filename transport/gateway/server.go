// Package gateway 提供 gRPC + HTTP (gRPC-Gateway) 双协议服务器.
package gateway

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/Tsukikage7/servex/v2/auth"
	authgrpcx "github.com/Tsukikage7/servex/v2/auth/grpcx"
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
	"github.com/Tsukikage7/servex/v2/transport/grpcx"
	"github.com/Tsukikage7/servex/v2/transport/health"
	"github.com/Tsukikage7/servex/v2/transport/response"
)

// Registrar 服务注册器接口.
type Registrar interface {
	// RegisterGRPC 注册 gRPC 服务.
	RegisterGRPC(server grpc.ServiceRegistrar)
	// RegisterGateway 注册 gRPC-Gateway 处理器.
	RegisterGateway(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error
}

// Server gRPC + HTTP 双协议服务器.
type Server struct {
	opts *options

	grpcServer   *grpc.Server
	grpcListener net.Listener

	httpServer  *http.Server
	httpHandler http.Handler
	mux         *runtime.ServeMux
	conn        *grpc.ClientConn

	// 内置健康检查
	health       *health.Health
	healthServer *health.GRPCServer
}

// New 创建 Gateway 服务器.
func New(opts ...Option) *Server {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.logger == nil {
		o.logger = logger.Nop()
	}
	o.logger = logger.WithComponent(o.logger, "Gateway")

	// 按照优先级顺序应用 gRPC 拦截器由外到内:
	// 1. Recovery
	// 2. Logging
	// 3. Tracing已在 WithObservability 或 WithConfig 中添加
	// 4. Metrics
	// 5. RateLimit
	// 6. ClientIP
	// 7. Tenant
	// 8. Auth在 applyAuthInterceptors 中添加
	applyNewInterceptors(o)

	// 应用 recovery 拦截器必须在所有 option 处理之后，放在拦截器链最前面
	applyRecoveryInterceptors(o)

	// 应用 auth 拦截器放在拦截器链末尾
	applyAuthInterceptors(o)

	muxOpts := []runtime.ServeMuxOption{
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions:   o.marshalOptions,
			UnmarshalOptions: protojson.UnmarshalOptions{DiscardUnknown: true},
		}),
	}

	// 如果启用统一响应，添加自定义错误处理器
	if o.enableResponse {
		muxOpts = append(muxOpts, runtime.WithErrorHandler(response.GatewayErrorHandler))
	}

	muxOpts = append(muxOpts, o.serveMuxOpts...)

	// 创建内置健康检查管理器
	healthOpts := []health.Option{health.WithTimeout(o.healthTimeout)}
	if o.version != "" {
		healthOpts = append(healthOpts, health.WithVersion(o.version))
	}
	healthOpts = append(healthOpts, o.healthOptions...)
	h := health.New(healthOpts...)

	return &Server{
		opts:   o,
		mux:    runtime.NewServeMux(muxOpts...),
		health: h,
	}
}

// Register 注册服务，支持链式调用.
func (s *Server) Register(services ...Registrar) *Server {
	s.opts.services = append(s.opts.services, services...)
	return s
}

// Start 启动服务器.
func (s *Server) Start(ctx context.Context) error {
	if err := s.startGRPC(); err != nil {
		return err
	}
	if err := s.connectGateway(); err != nil {
		return err
	}
	return s.startHTTP(ctx)
}

// Stop 停止服务器.
func (s *Server) Stop(ctx context.Context) error {
	var errs []error

	if s.httpServer != nil {
		s.opts.logger.With(logger.String("name", s.opts.name)).Info("HTTP 服务器正在停止")
		if err := s.httpServer.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if s.conn != nil {
		s.conn.Close()
	}

	if s.grpcServer != nil {
		s.opts.logger.With(logger.String("name", s.opts.name)).Info("gRPC 服务器正在停止")
		done := make(chan struct{})
		go func() {
			s.grpcServer.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
		case <-ctx.Done():
			s.grpcServer.Stop()
			errs = append(errs, ctx.Err())
		}
	}

	return errors.Join(errs...)
}

// Name 返回服务器名称.
func (s *Server) Name() string {
	return s.opts.name
}

// Addr 返回 gRPC 地址.
func (s *Server) Addr() string {
	return s.opts.grpcAddr
}

// HTTPAddr 返回 HTTP 地址.
func (s *Server) HTTPAddr() string {
	return s.opts.httpAddr
}

// GRPCServer 返回底层 gRPC Server.
func (s *Server) GRPCServer() *grpc.Server {
	return s.grpcServer
}

// Mux 返回底层 ServeMux.
func (s *Server) Mux() *runtime.ServeMux {
	return s.mux
}

// Health 返回健康检查管理器.
func (s *Server) Health() *health.Health {
	return s.health
}

// HealthEndpoint 返回健康检查端点信息.
//
// Gateway 使用 HTTP 健康检查通过 HTTP 端口.
func (s *Server) HealthEndpoint() *transport.HealthEndpoint {
	return &transport.HealthEndpoint{
		Type: transport.HealthCheckTypeHTTP,
		Addr: s.opts.httpAddr,
		Path: health.DefaultLivenessPath,
	}
}

// HealthServer 返回 gRPC 健康检查服务器.
func (s *Server) HealthServer() *health.GRPCServer {
	return s.healthServer
}

func (s *Server) startGRPC() error {
	lis, err := net.Listen("tcp", s.opts.grpcAddr)
	if err != nil {
		return err
	}
	s.grpcListener = lis

	serverOpts := []grpc.ServerOption{
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             s.opts.minPingInterval,
			PermitWithoutStream: true,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    s.opts.keepaliveTime,
			Timeout: s.opts.keepaliveTimeout,
		}),
	}
	// logger 注入拦截器最前面，保证所有拦截器和业务代码都能用 logger.FromContext
	allUnary := []grpc.UnaryServerInterceptor{loggerUnaryInterceptor(s.opts.logger)}
	allUnary = append(allUnary, s.opts.unaryInterceptors...)
	serverOpts = append(serverOpts, grpc.ChainUnaryInterceptor(allUnary...))

	allStream := []grpc.StreamServerInterceptor{loggerStreamInterceptor(s.opts.logger)}
	allStream = append(allStream, s.opts.streamInterceptors...)
	serverOpts = append(serverOpts, grpc.ChainStreamInterceptor(allStream...))
	serverOpts = append(serverOpts, s.opts.grpcServerOpts...)

	s.grpcServer = grpc.NewServer(serverOpts...)

	// 注册业务服务
	for _, svc := range s.opts.services {
		svc.RegisterGRPC(s.grpcServer)
	}

	// 注册 gRPC 健康检查服务
	s.healthServer = health.NewGRPCServer(s.health)
	s.healthServer.Register(s.grpcServer)

	// 启用 auth 时，扫描注册的服务并填充 proto 认证策略。
	// 没有 proto option 的方法默认不公开，仍按普通认证方法处理。
	if s.opts.discoveredPolicies != nil {
		s.discoverAuthPolicies()
	}

	if s.opts.enableReflection {
		reflection.Register(s.grpcServer)
	}

	s.opts.logger.With(
		logger.String("name", s.opts.name),
		logger.String("addr", s.opts.grpcAddr),
	).Info("gRPC 服务启动")

	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			s.opts.logger.With(
				logger.String("name", s.opts.name),
				logger.Err(err),
			).Error("gRPC 服务运行错误")
		}
	}()
	return nil
}

// discoverAuthPolicies 从注册的服务中发现 proto 认证策略.
func (s *Server) discoverAuthPolicies() {
	result := authgrpcx.DiscoverFromServer(s.grpcServer)

	for method, info := range result.MethodAuthInfos {
		s.opts.discoveredPolicies[method] = info
	}
	applyBuiltinAuthPolicies(s.opts.discoveredPolicies)

	s.opts.logger.With(
		logger.String("name", s.opts.name),
		logger.Int("policies", len(s.opts.discoveredPolicies)),
		logger.Int("public", countPublicPolicies(s.opts.discoveredPolicies)),
	).Info("自动发现认证策略")

	if len(result.PublicMethods) > 0 {
		for _, method := range result.PublicMethods {
			s.opts.logger.With(
				logger.String("method", method),
			).Debug("发现公开方法")
		}
	}
}

func applyBuiltinAuthPolicies(policies auth.MethodPolicyMap) {
	if policies == nil {
		return
	}
	policies["/grpc.health.v1.Health/Check"] = &auth.MethodAuthInfo{
		FullMethod: "/grpc.health.v1.Health/Check",
		Public:     true,
	}
	policies["/grpc.health.v1.Health/Watch"] = &auth.MethodAuthInfo{
		FullMethod: "/grpc.health.v1.Health/Watch",
		Public:     true,
	}
}

func countPublicPolicies(policies auth.MethodPolicyMap) int {
	count := 0
	for _, policy := range policies {
		if policy != nil && policy.Public {
			count++
		}
	}
	return count
}

func (s *Server) connectGateway() error {
	// 根据配置选择 gRPC 连接凭证
	dialOpts := make([]grpc.DialOption, 0, len(s.opts.dialOptions)+2)
	if s.opts.grpcTLSConfig != nil {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(s.opts.grpcTLSConfig)))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// 启用 tracing 时，注入 client interceptor 传播 trace contextHTTP span → gRPC metadata
	if s.opts.tracerName != "" {
		dialOpts = append(dialOpts, grpc.WithUnaryInterceptor(tracing.UnaryClientInterceptor(s.opts.tracerName)))
	}

	dialOpts = append(dialOpts, s.opts.dialOptions...)

	conn, err := grpc.NewClient(s.opts.grpcAddr, dialOpts...)
	if err != nil {
		return err
	}
	s.conn = conn

	ctx := context.Background()
	for _, svc := range s.opts.services {
		if err := svc.RegisterGateway(ctx, s.mux, conn); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) startHTTP(ctx context.Context) error {
	// 构建 HTTP 中间件链由内到外包装
	//
	// 最终请求执行顺序:
	// Recovery → Logging → Tracing → Metrics → CORS →
	// RateLimit → ClientIP → Tenant → Auth(via gRPC) → Health → handler
	var handler http.Handler = health.Middleware(s.health)(s.mux)

	// 统一成功响应格式紧贴 mux，捕获 gRPC-Gateway 输出后包裹为 {code,message,data}
	if s.opts.enableResponse {
		handler = response.GatewaySuccessResponseMiddleware(handler)
	}

	// 9. TenantHTTP 端
	if s.opts.tenantResolver != nil {
		tenantOpts := s.opts.tenantOpts
		if s.opts.logger != nil {
			tenantOpts = append(tenantOpts, tenant.WithLogger(s.opts.logger))
		}
		handler = tenant.HTTPMiddleware(s.opts.tenantResolver, tenantOpts...)(handler)
	}

	// 8. ClientIPHTTP 端
	if s.opts.enableClientIP {
		handler = clientip.HTTPMiddleware(s.opts.clientIPOpts...)(handler)
	}

	// 7. RateLimitHTTP 端
	if s.opts.rateLimiter != nil {
		handler = ratelimit.HTTPMiddleware(s.opts.rateLimiter)(handler)
	}

	// 6. CORS仅 HTTP 端
	if s.opts.enableCORS {
		handler = cors.HTTPMiddleware(s.opts.corsOpts...)(handler)
	}

	// 5. MetricsHTTP 端
	if s.opts.metricsCollector != nil {
		handler = metrics.HTTPMiddleware(s.opts.metricsCollector)(handler)
		// 注册 /metrics 端点
		s.mux.HandlePath("GET", s.opts.metricsCollector.GetPath(), func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
			s.opts.metricsCollector.GetHandler().ServeHTTP(w, r)
		})
	}

	// 4. TracingHTTP 端
	if s.opts.tracerName != "" {
		handler = tracing.HTTPMiddleware(s.opts.tracerName, s.opts.tracingSkipPaths...)(handler)
	}

	// 3. LoggingHTTP 端
	if s.opts.enableLogging && s.opts.logger != nil {
		handler = logging.HTTPMiddleware(
			logging.WithLogger(s.opts.logger),
			logging.WithSkipPaths(s.opts.loggingSkipPaths...),
		)(handler)
	}

	// 1. RecoveryHTTP 端，最外层
	if s.opts.enableRecovery {
		handler = recovery.HTTPMiddleware(recovery.WithLogger(s.opts.logger))(handler)
	}

	// 应用用户自定义 HTTP 中间件
	for i := len(s.opts.httpMiddlewares) - 1; i >= 0; i-- {
		handler = s.opts.httpMiddlewares[i](handler)
	}

	// logger 注入最外层，保证所有中间件和业务代码都能用 logger.FromContext(ctx)
	handler = injectLogger(s.opts.logger)(handler)

	s.httpHandler = handler

	s.httpServer = &http.Server{
		Addr:         s.opts.httpAddr,
		Handler:      handler,
		ReadTimeout:  s.opts.httpReadTimeout,
		WriteTimeout: s.opts.httpWriteTimeout,
		IdleTimeout:  s.opts.httpIdleTimeout,
		TLSConfig:    s.opts.httpTLSConfig,
	}

	s.opts.logger.With(
		logger.String("name", s.opts.name),
		logger.String("addr", s.opts.httpAddr),
	).Info("HTTP 服务启动")

	errCh := make(chan error, 1)
	go func() {
		var err error
		if s.opts.httpTLSConfig != nil {
			err = s.httpServer.ListenAndServeTLS("", "")
		} else {
			err = s.httpServer.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// 短暂等待以捕获启动时的立即错误如端口占用
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	case <-time.After(100 * time.Millisecond):
	}
	return nil
}

// 确保 Server 实现了 transport.HealthCheckable 接口.
var _ transport.HealthCheckable = (*Server)(nil)

// injectLogger 返回 HTTP 中间件，将 logger 注入到每个请求的 context 中.
func injectLogger(log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := logger.NewContext(r.Context(), log)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// loggerUnaryInterceptor 将 logger 注入到每个 gRPC 请求的 context 中.
func loggerUnaryInterceptor(log logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx = logger.NewContext(ctx, log)
		return handler(ctx, req)
	}
}

// loggerStreamInterceptor 将 logger 注入到每个 gRPC 流的 context 中.
func loggerStreamInterceptor(log logger.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := logger.NewContext(ss.Context(), log)
		wrapped := grpcx.WrapServerStream(ss, ctx)
		return handler(srv, wrapped)
	}
}
