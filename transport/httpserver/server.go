// Package httpserver 提供 HTTP 服务器实现.
package httpserver

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/pprof"
	"slices"
	"strings"
	"time"

	"github.com/Tsukikage7/servex/auth"
	"github.com/Tsukikage7/servex/httpx/clientip"
	"github.com/Tsukikage7/servex/middleware/logging"
	"github.com/Tsukikage7/servex/middleware/recovery"
	"github.com/Tsukikage7/servex/observability/logger"
	"github.com/Tsukikage7/servex/observability/tracing"
	"github.com/Tsukikage7/servex/tenant"
	"github.com/Tsukikage7/servex/transport"
	"github.com/Tsukikage7/servex/transport/health"
)

// Server HTTP 服务器.
type Server struct {
	opts    *options
	router  *Router // 原始路由器引用，用于 Register 注册服务
	handler http.Handler
	server  *http.Server
	health  *health.Health
}

// New 创建 HTTP 服务器.
//
// 示例:
//
//	server := httpserver.New(mux,
//	    httpserver.WithLogger(log),
//	    httpserver.WithAddr(":8080"),
//	    httpserver.WithAuth(authenticator, "/api/login", "/api/register"),
//	    httpserver.WithRecovery(),
//	    httpserver.WithProfiling("/debug/pprof"),
//	)
func New(handler http.Handler, opts ...Option) *Server {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.logger == nil {
		panic("http server: logger is required")
	}

	// 创建健康检查
	healthOpts := []health.Option{health.WithTimeout(o.healthTimeout)}
	healthOpts = append(healthOpts, o.healthOptions...)
	h := health.New(healthOpts...)

	// 保存原始 Router 引用（如果传入的是 Router），用于后续 Register 调用
	var router *Router
	if r, ok := handler.(*Router); ok {
		router = r
	}

	// 应用用户自定义中间件（按声明顺序执行，最先声明的最先被请求触达）
	for _, mw := range slices.Backward(o.middlewares) {
		handler = mw(handler)
	}

	// 中间件包装（由内到外）
	wrapped := health.Middleware(h)(handler)

	if o.clientIP {
		wrapped = clientip.HTTPMiddleware(o.clientIPOpts...)(wrapped)
	}

	if o.tenantResolver != nil {
		wrapped = tenant.HTTPMiddleware(o.tenantResolver, o.tenantOpts...)(wrapped)
	}

	if o.authenticator != nil {
		wrapped = auth.HTTPMiddleware(o.authenticator, o.authOpts...)(wrapped)
	}

	if o.traceName != "" {
		wrapped = tracing.HTTPMiddleware(o.traceName)(wrapped)
	}

	if o.recovery {
		wrapped = recovery.HTTPMiddleware(recovery.WithLogger(o.logger))(wrapped)
	}

	if o.loggingEnabled {
		wrapped = logging.HTTPMiddleware(
			logging.WithLogger(o.logger),
			logging.WithSkipPaths(o.loggingSkipPaths...),
		)(wrapped)
	}

	if o.profiling != "" {
		wrapped = wrapProfiling(wrapped, o.profiling, o.profilingAuth)
	}

	return &Server{opts: o, router: router, handler: wrapped, health: h}
}

func wrapProfiling(next http.Handler, prefix string, authFn func(*http.Request) bool) http.Handler {
	prefix = strings.TrimSuffix(prefix, "/")
	mux := http.NewServeMux()

	// 认证包装器
	wrap := func(h http.HandlerFunc) http.HandlerFunc {
		if authFn == nil {
			return h
		}
		return func(w http.ResponseWriter, r *http.Request) {
			if !authFn(r) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			h(w, r)
		}
	}

	// 注册 pprof 端点
	mux.HandleFunc(prefix+"/", wrap(pprof.Index))
	mux.HandleFunc(prefix+"/cmdline", wrap(pprof.Cmdline))
	mux.HandleFunc(prefix+"/profile", wrap(pprof.Profile))
	mux.HandleFunc(prefix+"/symbol", wrap(pprof.Symbol))
	mux.HandleFunc(prefix+"/trace", wrap(pprof.Trace))
	mux.HandleFunc(prefix+"/heap", wrap(func(w http.ResponseWriter, r *http.Request) {
		pprof.Handler("heap").ServeHTTP(w, r)
	}))
	mux.HandleFunc(prefix+"/goroutine", wrap(func(w http.ResponseWriter, r *http.Request) {
		pprof.Handler("goroutine").ServeHTTP(w, r)
	}))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, prefix) {
			mux.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Start 启动服务器.
func (s *Server) Start(ctx context.Context) error {
	s.server = &http.Server{
		Addr:         s.opts.addr,
		Handler:      s.handler,
		ReadTimeout:  s.opts.readTimeout,
		WriteTimeout: s.opts.writeTimeout,
		IdleTimeout:  s.opts.idleTimeout,
		TLSConfig:    s.opts.tlsConfig,
	}

	s.opts.logger.With(
		logger.String("name", s.opts.name),
		logger.String("addr", s.opts.addr),
	).Info("[HTTP] 服务器启动")

	errCh := make(chan error, 1)
	if s.opts.tlsConfig != nil {
		// TLS 模式：证书已通过 TLSConfig 加载，传空字符串
		go func() { errCh <- s.server.ListenAndServeTLS("", "") }()
	} else {
		go func() { errCh <- s.server.ListenAndServe() }()
	}

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	case <-ctx.Done():
	}
	return nil
}

// Stop 停止服务器.
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	s.opts.logger.With(logger.String("name", s.opts.name)).Info("[HTTP] 服务器停止")
	return s.server.Shutdown(ctx)
}

// Name 返回服务器名称.
func (s *Server) Name() string { return s.opts.name }

// Addr 返回监听地址.
func (s *Server) Addr() string { return s.opts.addr }

// Handler 返回 HTTP 处理器.
func (s *Server) Handler() http.Handler { return s.handler }

// Health 返回健康检查管理器.
func (s *Server) Health() *health.Health { return s.health }

// HealthEndpoint 返回健康检查端点信息.
func (s *Server) HealthEndpoint() *transport.HealthEndpoint {
	return &transport.HealthEndpoint{
		Type: transport.HealthCheckTypeHTTP,
		Addr: s.opts.addr,
		Path: health.DefaultLivenessPath,
	}
}

// Option 配置选项函数.
type Option func(*options)

type options struct {
	name         string
	addr         string
	readTimeout  time.Duration
	writeTimeout time.Duration
	idleTimeout  time.Duration
	logger       logger.Logger

	// Health
	healthTimeout time.Duration
	healthOptions []health.Option

	// Middleware
	recovery         bool
	loggingEnabled   bool
	loggingSkipPaths []string
	traceName        string
	clientIP         bool
	clientIPOpts     []clientip.Option
	authenticator    auth.Authenticator
	authOpts         []auth.Option
	tenantResolver   tenant.Resolver
	tenantOpts       []tenant.Option
	profiling        string
	profilingAuth    func(*http.Request) bool

	// 用户自定义中间件
	middlewares []func(http.Handler) http.Handler

	// TLS
	tlsConfig *tls.Config
}

func defaultOptions() *options {
	return &options{
		name:          "HTTP",
		addr:          ":8080",
		readTimeout:   30 * time.Second,
		writeTimeout:  30 * time.Second,
		idleTimeout:   120 * time.Second,
		healthTimeout: 5 * time.Second,
	}
}

// WithLogger 设置日志记录器（必需）.
func WithLogger(l logger.Logger) Option {
	return func(o *options) { o.logger = l }
}

// WithName 设置服务器名称.
func WithName(name string) Option {
	return func(o *options) { o.name = name }
}

// WithAddr 设置监听地址.
func WithAddr(addr string) Option {
	return func(o *options) { o.addr = addr }
}

// WithTimeout 设置超时时间.
func WithTimeout(read, write, idle time.Duration) Option {
	return func(o *options) {
		if read > 0 {
			o.readTimeout = read
		}
		if write > 0 {
			o.writeTimeout = write
		}
		if idle > 0 {
			o.idleTimeout = idle
		}
	}
}

// WithRecovery 启用 panic 恢复.
func WithRecovery() Option {
	return func(o *options) { o.recovery = true }
}

// WithLogging 启用 HTTP 请求访问日志.
//
// 日志中间件位于 recovery 之外，可记录每个请求的方法、路径、状态码、耗时和响应字节数.
// 可通过 skipPaths 跳过不需要记录的路径（如 /health、/metrics）.
//
// 示例:
//
//	httpserver.New(mux,
//	    httpserver.WithLogging("/health", "/metrics"),
//	)
func WithLogging(skipPaths ...string) Option {
	return func(o *options) {
		o.loggingEnabled = true
		o.loggingSkipPaths = skipPaths
	}
}

// WithTrace 启用链路追踪.
func WithTrace(serviceName string) Option {
	return func(o *options) { o.traceName = serviceName }
}

// WithClientIP 启用客户端 IP 提取.
func WithClientIP(opts ...clientip.Option) Option {
	return func(o *options) {
		o.clientIP = true
		o.clientIPOpts = opts
	}
}

// WithAuth 启用认证，可选指定公开路径.
//
// 示例:
//
//	httpserver.WithAuth(authenticator)                           // 所有路径都需认证
//	httpserver.WithAuth(authenticator, "/login", "/register")    // 指定公开路径
//	httpserver.WithAuth(authenticator, "/api/public/*")          // 前缀匹配
func WithAuth(authenticator auth.Authenticator, publicPaths ...string) Option {
	return func(o *options) {
		o.authenticator = authenticator
		if o.logger != nil {
			o.authOpts = append(o.authOpts, auth.WithLogger(o.logger))
		}
		if len(publicPaths) > 0 {
			o.authOpts = append(o.authOpts, auth.WithSkipper(buildPathSkipper(publicPaths)))
		}
	}
}

// WithTenant 启用多租户解析.
//
// 租户中间件位于 auth 之后（更靠近 handler），确保先完成认证再解析租户.
//
// 示例:
//
//	httpserver.New(mux,
//	    httpserver.WithAuth(authenticator),
//	    httpserver.WithTenant(resolver,
//	        tenant.WithTokenExtractor(tenant.HeaderTokenExtractor("X-Tenant-ID")),
//	    ),
//	)
func WithTenant(resolver tenant.Resolver, opts ...tenant.Option) Option {
	return func(o *options) {
		o.tenantResolver = resolver
		o.tenantOpts = opts
		if o.logger != nil {
			o.tenantOpts = append(o.tenantOpts, tenant.WithLogger(o.logger))
		}
	}
}

// WithProfiling 启用 pprof 端点.
//
// 示例:
//
//	httpserver.WithProfiling("/debug/pprof")
func WithProfiling(pathPrefix string) Option {
	return func(o *options) { o.profiling = pathPrefix }
}

// WithMiddlewares 注册用户自定义 HTTP 中间件.
//
// 中间件按声明顺序执行：WithMiddlewares(cors, ratelimit) 的执行顺序为
// cors → ratelimit → 路由，位于框架内置中间件（recovery、认证等）之后.
//
// 示例:
//
//	httpserver.New(mux,
//	    httpserver.WithMiddlewares(
//	        cors.HTTPMiddleware(cors.WithAllowOrigins("*")),
//	        ratelimit.HTTPMiddleware(limiter),
//	        requestid.HTTPMiddleware(),
//	    ),
//	)
func WithMiddlewares(mws ...func(http.Handler) http.Handler) Option {
	return func(o *options) {
		o.middlewares = append(o.middlewares, mws...)
	}
}

// WithProfilingAuth 启用带认证的 pprof 端点.
func WithProfilingAuth(pathPrefix string, authFn func(*http.Request) bool) Option {
	return func(o *options) {
		o.profiling = pathPrefix
		o.profilingAuth = authFn
	}
}

// WithTLS 启用 TLS.
//
// 传入 *tls.Config 后，服务器启动时将使用 ListenAndServeTLS 而非 ListenAndServe.
// 可配合 transport/tls (tlsx) 包生成配置：
//
//	tlsCfg, _ := tlsx.NewServerTLSConfig(&tlsx.Config{
//	    CertFile: "server.crt",
//	    KeyFile:  "server.key",
//	})
//	httpserver.New(mux, httpserver.WithTLS(tlsCfg))
func WithTLS(cfg *tls.Config) Option {
	return func(o *options) {
		o.tlsConfig = cfg
	}
}

// WithHealthTimeout 设置健康检查超时.
func WithHealthTimeout(d time.Duration) Option {
	return func(o *options) { o.healthTimeout = d }
}

// WithHealthChecker 添加健康检查器.
func WithHealthChecker(checkers ...health.Checker) Option {
	return func(o *options) {
		o.healthOptions = append(o.healthOptions, health.WithReadinessChecker(checkers...))
	}
}

// buildPathSkipper 构建路径跳过器.
func buildPathSkipper(paths []string) auth.Skipper {
	exact := make(map[string]bool)
	var prefixes []string

	for _, p := range paths {
		if len(p) > 0 && p[len(p)-1] == '*' {
			prefixes = append(prefixes, p[:len(p)-1])
		} else {
			exact[p] = true
		}
	}

	return func(_ context.Context, req any) bool {
		r, ok := req.(*http.Request)
		if !ok {
			return false
		}
		if exact[r.URL.Path] {
			return true
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				return true
			}
		}
		return false
	}
}

// Registrar HTTP 服务注册器接口.
type Registrar interface {
	RegisterHTTP(router *Router)
}

// Register 注册 HTTP 服务，支持链式调用.
//
// 如果 New 传入的 handler 是 *Router，则直接使用该 Router 注册路由.
// 否则 Register 会创建新 Router 并将原 handler 作为兜底路由挂载.
//
// 示例:
//
//	router := httpserver.NewRouter()
//	srv := httpserver.New(router, httpserver.WithLogger(log))
//	srv.Register(userService, orderService)
func (s *Server) Register(registrars ...Registrar) *Server {
	if s.router == nil {
		s.router = NewRouter()
	}
	for _, r := range registrars {
		r.RegisterHTTP(s.router)
	}
	return s
}

var _ transport.HealthCheckable = (*Server)(nil)
