// Package grpcserver 提供 gRPC 服务器实现.
package grpcserver

import (
	"context"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	"github.com/Tsukikage7/servex/auth"
	"github.com/Tsukikage7/servex/httpx/clientip"
	"github.com/Tsukikage7/servex/middleware/logging"
	"github.com/Tsukikage7/servex/middleware/ratelimit"
	"github.com/Tsukikage7/servex/middleware/recovery"
	"github.com/Tsukikage7/servex/observability/logger"
	"github.com/Tsukikage7/servex/observability/metrics"
	"github.com/Tsukikage7/servex/tenant"
	"github.com/Tsukikage7/servex/transport"
	"github.com/Tsukikage7/servex/transport/health"
)

// Registrar gRPC 服务注册器接口.
type Registrar interface {
	RegisterGRPC(server *grpc.Server)
}

// Server gRPC 服务器.
type Server struct {
	opts     *options
	server   *grpc.Server
	listener net.Listener

	// 内置健康检查
	health       *health.Health
	healthServer *health.GRPCServer
}

// New 创建 gRPC 服务器，如果未设置 logger 会 panic.
func New(opts ...Option) *Server {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.logger == nil {
		panic("grpc server: 必须设置 logger")
	}

	// 创建内置健康检查管理器
	healthOpts := []health.Option{health.WithTimeout(o.healthTimeout)}
	healthOpts = append(healthOpts, o.healthOptions...)
	h := health.New(healthOpts...)

	return &Server{
		opts:   o,
		health: h,
	}
}

// Register 注册 gRPC 服务，支持链式调用.
func (s *Server) Register(services ...Registrar) *Server {
	s.opts.services = append(s.opts.services, services...)
	return s
}

// GRPCServer 返回底层 grpc.Server，未启动时返回 nil.
func (s *Server) GRPCServer() *grpc.Server {
	return s.server
}

// Health 返回健康检查管理器.
func (s *Server) Health() *health.Health {
	return s.health
}

// HealthEndpoint 返回健康检查端点信息.
func (s *Server) HealthEndpoint() *transport.HealthEndpoint {
	return &transport.HealthEndpoint{
		Type: transport.HealthCheckTypeGRPC,
		Addr: s.opts.addr,
	}
}

// HealthServer 返回 gRPC 健康检查服务器.
func (s *Server) HealthServer() *health.GRPCServer {
	return s.healthServer
}

// Start 启动 gRPC 服务器.
func (s *Server) Start(ctx context.Context) error {
	// 创建监听器
	listener, err := net.Listen("tcp", s.opts.addr)
	if err != nil {
		return err
	}
	s.listener = listener

	// 构建服务器选项
	serverOpts := s.buildServerOptions()

	// 创建 gRPC 服务器
	s.server = grpc.NewServer(serverOpts...)

	// 注册所有业务服务
	for _, service := range s.opts.services {
		service.RegisterGRPC(s.server)
	}

	// 注册 gRPC 健康检查服务
	s.healthServer = health.NewGRPCServer(s.health)
	s.healthServer.Register(s.server)

	// 如果启用自动发现，扫描注册的服务并填充 discoveredMethods
	if s.opts.enableAutoDiscovery && s.opts.discoveredMethods != nil {
		s.discoverPublicMethods()
	}

	// 启用反射
	if s.opts.enableReflection {
		reflection.Register(s.server)
	}

	s.opts.logger.With(
		logger.String("name", s.opts.name),
		logger.String("addr", s.opts.addr),
	).Info("[gRPC] 服务器启动")

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.server.Serve(listener)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		// 上下文取消，正常退出
	}

	return nil
}

// Stop 停止 gRPC 服务器.
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	s.opts.logger.With(logger.String("name", s.opts.name)).Info("[gRPC] 服务器停止中")

	// 优雅关闭
	done := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		// 超时，强制关闭
		s.server.Stop()
		return ctx.Err()
	}
}

// Name 返回服务器名称.
func (s *Server) Name() string {
	return s.opts.name
}

// Addr 返回服务器地址.
func (s *Server) Addr() string {
	return s.opts.addr
}

// buildServerOptions 构建 gRPC 服务器选项.
func (s *Server) buildServerOptions() []grpc.ServerOption {
	opts := []grpc.ServerOption{
		// Keepalive 执行策略（防止客户端 ping 过于频繁）
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             s.opts.minPingInterval,
			PermitWithoutStream: true,
		}),
		// Keepalive 服务端参数（主动检测客户端健康）
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    s.opts.keepaliveTime,
			Timeout: s.opts.keepaliveTimeout,
		}),
	}

	// 构建拦截器链
	unaryInterceptors := s.opts.unaryInterceptors
	streamInterceptors := s.opts.streamInterceptors

	// 如果启用指标收集，添加 metrics 拦截器
	if s.opts.metricsCollector != nil {
		unaryInterceptors = append(
			unaryInterceptors,
			metrics.UnaryServerInterceptor(s.opts.metricsCollector),
		)
		streamInterceptors = append(
			streamInterceptors,
			metrics.StreamServerInterceptor(s.opts.metricsCollector),
		)
	}

	// 如果启用限流，添加 ratelimit 拦截器
	if s.opts.rateLimiter != nil {
		unaryInterceptors = append(
			unaryInterceptors,
			ratelimit.UnaryServerInterceptor(s.opts.rateLimiter),
		)
		streamInterceptors = append(
			streamInterceptors,
			ratelimit.StreamServerInterceptor(s.opts.rateLimiter),
		)
	}

	// 如果启用客户端 IP 提取，添加 clientip 拦截器
	if s.opts.enableClientIP {
		unaryInterceptors = append(
			unaryInterceptors,
			clientip.UnaryServerInterceptor(s.opts.clientIPOptions...),
		)
		streamInterceptors = append(
			streamInterceptors,
			clientip.StreamServerInterceptor(s.opts.clientIPOptions...),
		)
	}

	// 如果启用认证，添加 auth 拦截器
	if s.opts.authenticator != nil {
		authOpts := s.buildAuthOptions()
		unaryInterceptors = append(
			unaryInterceptors,
			auth.UnaryServerInterceptor(s.opts.authenticator, authOpts...),
		)
		streamInterceptors = append(
			streamInterceptors,
			auth.StreamServerInterceptor(s.opts.authenticator, authOpts...),
		)
	}

	// 如果启用租户解析，添加 tenant 拦截器（在 auth 之后）
	if s.opts.tenantResolver != nil {
		tenantOpts := s.buildTenantOptions()
		unaryInterceptors = append(
			unaryInterceptors,
			tenant.UnaryServerInterceptor(s.opts.tenantResolver, tenantOpts...),
		)
		streamInterceptors = append(
			streamInterceptors,
			tenant.StreamServerInterceptor(s.opts.tenantResolver, tenantOpts...),
		)
	}

	// 如果启用 panic 恢复，将 recovery 拦截器添加到最前面（最外层）
	if s.opts.enableRecovery {
		unaryInterceptors = append(
			[]grpc.UnaryServerInterceptor{recovery.UnaryServerInterceptor(recovery.WithLogger(s.opts.logger))},
			unaryInterceptors...,
		)
		streamInterceptors = append(
			[]grpc.StreamServerInterceptor{recovery.StreamServerInterceptor(recovery.WithLogger(s.opts.logger))},
			streamInterceptors...,
		)
	}

	// 如果启用日志，将 logging 拦截器添加到最前面（最外层，包裹 recovery）
	if s.opts.enableLogging {
		logOpts := []logging.Option{logging.WithLogger(s.opts.logger)}
		if len(s.opts.loggingSkipPaths) > 0 {
			logOpts = append(logOpts, logging.WithSkipPaths(s.opts.loggingSkipPaths...))
		}
		unaryInterceptors = append(
			[]grpc.UnaryServerInterceptor{logging.UnaryServerInterceptor(logOpts...)},
			unaryInterceptors...,
		)
		streamInterceptors = append(
			[]grpc.StreamServerInterceptor{logging.StreamServerInterceptor(logOpts...)},
			streamInterceptors...,
		)
	}

	// 添加拦截器
	if len(unaryInterceptors) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(unaryInterceptors...))
	}

	if len(streamInterceptors) > 0 {
		opts = append(opts, grpc.ChainStreamInterceptor(streamInterceptors...))
	}

	// 添加自定义选项
	opts = append(opts, s.opts.serverOptions...)

	// 添加 TLS 凭据
	if s.opts.tlsConfig != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(s.opts.tlsConfig)))
	}

	return opts
}

// buildAuthOptions 构建 auth 选项.
func (s *Server) buildAuthOptions() []auth.Option {
	var authOpts []auth.Option

	// 添加 logger
	if s.opts.logger != nil {
		authOpts = append(authOpts, auth.WithLogger(s.opts.logger))
	}

	// 添加用户配置的选项
	authOpts = append(authOpts, s.opts.authOptions...)

	// 构建 skipper
	if len(s.opts.publicMethods) > 0 {
		authOpts = append(authOpts, auth.WithSkipper(buildMethodSkipper(s.opts.publicMethods)))
	}

	return authOpts
}

// buildTenantOptions 构建 tenant 选项.
func (s *Server) buildTenantOptions() []tenant.Option {
	var tenantOpts []tenant.Option

	if s.opts.logger != nil {
		tenantOpts = append(tenantOpts, tenant.WithLogger(s.opts.logger))
	}

	tenantOpts = append(tenantOpts, s.opts.tenantOptions...)
	return tenantOpts
}

// discoverPublicMethods 从注册的服务中发现公开方法.
func (s *Server) discoverPublicMethods() {
	result := auth.DiscoverFromServer(s.server)

	// 填充 discoveredMethods map
	for _, method := range result.PublicMethods {
		s.opts.discoveredMethods[method] = true
	}

	if len(result.PublicMethods) > 0 {
		s.opts.logger.With(
			logger.String("name", s.opts.name),
			logger.Int("count", len(result.PublicMethods)),
		).Info("[gRPC] 自动发现公开方法")

		for _, method := range result.PublicMethods {
			s.opts.logger.With(
				logger.String("name", s.opts.name),
				logger.String("method", method),
			).Debug("[gRPC] 发现公开方法")
		}
	}
}

// 确保 Server 实现了 transport.HealthCheckable 接口.
var _ transport.HealthCheckable = (*Server)(nil)
