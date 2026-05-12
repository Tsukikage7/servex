// Package connectserver 提供可选的 Connect RPC HTTP 服务器.
package connectserver

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/transport"
	"github.com/Tsukikage7/servex/v2/transport/health"
)

// Registrar Connect 服务注册器接口.
type Registrar interface {
	RegisterConnect(mux *http.ServeMux, opts ...connect.HandlerOption)
}

// Server Connect RPC HTTP 服务器.
type Server struct {
	opts    *options
	mux     *http.ServeMux
	handler http.Handler
	server  *http.Server
	health  *health.Health
	once    sync.Once
}

// New 创建 Connect RPC HTTP 服务器.
func New(opts ...Option) *Server {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.logger == nil {
		o.logger = logger.Nop()
	}
	o.logger = logger.WithComponent(o.logger, "Connect")

	healthOpts := []health.Option{health.WithTimeout(o.healthTimeout)}
	if o.version != "" {
		healthOpts = append(healthOpts, health.WithVersion(o.version))
	}
	healthOpts = append(healthOpts, o.healthOptions...)
	h := health.New(healthOpts...)

	if o.recovery {
		o.handlerOptions = append([]connect.HandlerOption{
			connect.WithRecover(func(_ context.Context, spec connect.Spec, _ http.Header, v any) error {
				o.logger.With(
					logger.String("procedure", spec.Procedure),
					logger.Any("panic", v),
				).Error("RPC 异常已恢复")
				return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
			}),
		}, o.handlerOptions...)
	}

	mux := http.NewServeMux()
	var handler http.Handler = mux

	for _, mw := range slices.Backward(o.middlewares) {
		handler = mw(handler)
	}
	if o.logging {
		handler = loggingMiddleware(o.logger, o.skipLogPaths)(handler)
	}
	handler = health.Middleware(h)(handler)
	handler = injectLogger(o.logger)(handler)

	return &Server{opts: o, mux: mux, handler: handler, health: h}
}

// Register 注册 Connect 服务，支持链式调用.
func (s *Server) Register(registrars ...Registrar) *Server {
	for _, r := range registrars {
		r.RegisterConnect(s.mux, s.opts.handlerOptions...)
	}
	return s
}

// Mux 返回底层 HTTP mux.
func (s *Server) Mux() *http.ServeMux { return s.mux }

// Handler 返回包装后的 HTTP 处理器.
func (s *Server) Handler() http.Handler { return s.handler }

// Start 启动服务器.
func (s *Server) Start(ctx context.Context) error {
	var startErr error
	s.once.Do(func() {
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
		).Info("服务器启动")

		errCh := make(chan error, 1)
		if s.opts.tlsConfig != nil {
			go func() { errCh <- s.server.ListenAndServeTLS("", "") }()
		} else {
			go func() { errCh <- s.server.ListenAndServe() }()
		}

		select {
		case err := <-errCh:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				startErr = err
			}
		case <-ctx.Done():
		}
	})
	return startErr
}

// Stop 停止服务器.
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	s.opts.logger.With(logger.String("name", s.opts.name)).Info("服务器停止")
	return s.server.Shutdown(ctx)
}

// Name 返回服务器名称.
func (s *Server) Name() string { return s.opts.name }

// Addr 返回监听地址.
func (s *Server) Addr() string { return s.opts.addr }

// Health 返回健康检查管理器.
func (s *Server) Health() *health.Health { return s.health }

// HealthEndpoint 返回 HTTP 健康检查端点信息.
func (s *Server) HealthEndpoint() *transport.HealthEndpoint {
	return &transport.HealthEndpoint{
		Type: transport.HealthCheckTypeHTTP,
		Addr: s.opts.addr,
		Path: health.DefaultLivenessPath,
	}
}

func injectLogger(log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := logger.NewContext(r.Context(), log)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func loggingMiddleware(log logger.Logger, skipPaths []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if shouldSkip(r.URL.Path, skipPaths) {
				next.ServeHTTP(w, r)
				return
			}

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			next.ServeHTTP(rec, r)

			log.With(
				logger.String("method", r.Method),
				logger.String("path", r.URL.Path),
				logger.Int("status", rec.status),
				logger.Duration("elapsed", time.Since(start)),
			).Info("请求完成")
		})
	}
}

func shouldSkip(path string, skipPaths []string) bool {
	for _, p := range skipPaths {
		if p == path {
			return true
		}
	}
	return false
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

var _ transport.HealthCheckable = (*Server)(nil)
