package connectserver

import (
	"crypto/tls"
	"net/http"
	"time"

	"connectrpc.com/connect"

	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/transport"
	"github.com/Tsukikage7/servex/v2/transport/health"
)

// Option 配置选项函数.
type Option func(*options)

type options struct {
	name           string
	version        string
	addr           string
	readTimeout    time.Duration
	writeTimeout   time.Duration
	idleTimeout    time.Duration
	logger         logger.Logger
	middlewares    []func(http.Handler) http.Handler
	handlerOptions []connect.HandlerOption
	logging        bool
	skipLogPaths   []string
	recovery       bool
	healthTimeout  time.Duration
	healthOptions  []health.Option
	tlsConfig      *tls.Config
}

func defaultOptions() *options {
	return &options{
		name:          "Connect",
		addr:          ":8080",
		readTimeout:   30 * time.Second,
		writeTimeout:  30 * time.Second,
		idleTimeout:   120 * time.Second,
		healthTimeout: 5 * time.Second,
	}
}

// WithName 设置服务器名称.
func WithName(name string) Option {
	return func(o *options) { o.name = name }
}

// WithVersion 设置服务版本，会注入到健康检查响应中.
func WithVersion(v string) Option {
	return func(o *options) { o.version = v }
}

// WithAddr 设置监听地址.
func WithAddr(addr string) Option {
	return func(o *options) { o.addr = addr }
}

// WithConfig 应用通用 HTTP 服务器配置.
func WithConfig(cfg transport.HTTPConfig) Option {
	return func(o *options) {
		if cfg.Name != "" {
			o.name = cfg.Name
		}
		if cfg.Addr != "" {
			o.addr = cfg.Addr
		}
		if cfg.ReadTimeout > 0 {
			o.readTimeout = cfg.ReadTimeout
		}
		if cfg.WriteTimeout > 0 {
			o.writeTimeout = cfg.WriteTimeout
		}
		if cfg.IdleTimeout > 0 {
			o.idleTimeout = cfg.IdleTimeout
		}
	}
}

// WithTimeout 设置 HTTP 服务器超时.
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

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option {
	return func(o *options) { o.logger = log }
}

// WithMiddlewares 添加 HTTP 中间件.
func WithMiddlewares(mws ...func(http.Handler) http.Handler) Option {
	return func(o *options) {
		o.middlewares = append(o.middlewares, mws...)
	}
}

// WithHandlerOptions 设置生成的 Connect handler 选项.
func WithHandlerOptions(opts ...connect.HandlerOption) Option {
	return func(o *options) {
		o.handlerOptions = append(o.handlerOptions, opts...)
	}
}

// WithLogging 启用请求日志.
func WithLogging(skipPaths ...string) Option {
	return func(o *options) {
		o.logging = true
		o.skipLogPaths = append(o.skipLogPaths, skipPaths...)
	}
}

// WithRecovery 启用 Connect RPC panic 恢复.
func WithRecovery() Option {
	return func(o *options) { o.recovery = true }
}

// WithHealthTimeout 设置健康检查超时.
func WithHealthTimeout(d time.Duration) Option {
	return func(o *options) { o.healthTimeout = d }
}

// WithHealthChecker 添加就绪检查器.
func WithHealthChecker(checkers ...health.Checker) Option {
	return func(o *options) {
		o.healthOptions = append(o.healthOptions, health.WithReadinessChecker(checkers...))
	}
}

// WithTLS 启用 HTTPS.
func WithTLS(cfg *tls.Config) Option {
	return func(o *options) { o.tlsConfig = cfg }
}
