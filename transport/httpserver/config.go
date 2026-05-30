package httpserver

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Tsukikage7/servex/v2/middleware/logging"
	"github.com/Tsukikage7/servex/v2/middleware/recovery"
	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/transport"
	tlsx "github.com/Tsukikage7/servex/v2/transport/tls"
)

// Config HTTP 服务器配置.
type Config struct {
	Name         string                  `json:"name" yaml:"name" mapstructure:"name"`
	Addr         string                  `json:"addr" yaml:"addr" mapstructure:"addr"`
	ReadTimeout  time.Duration           `json:"read_timeout" yaml:"read_timeout" mapstructure:"read_timeout"`
	WriteTimeout time.Duration           `json:"write_timeout" yaml:"write_timeout" mapstructure:"write_timeout"`
	IdleTimeout  time.Duration           `json:"idle_timeout" yaml:"idle_timeout" mapstructure:"idle_timeout"`
	TLS          *tlsx.Config            `json:"tls" yaml:"tls" mapstructure:"tls"`
	Profiling    string                  `json:"profiling" yaml:"profiling" mapstructure:"profiling"` // 路径前缀，空字符串表示不启用
	Runtime      transport.RuntimeConfig `json:"runtime" yaml:"runtime" mapstructure:"runtime"`
	Logging      transport.LoggingConfig `json:"logging" yaml:"logging" mapstructure:"logging"`
}

// NewFromTransportConfig 从 transport.HTTPConfig 创建 HTTP 服务器。
func NewFromTransportConfig(handler http.Handler, cfg transport.HTTPConfig, log logger.Logger, additionalOpts ...Option) (*Server, error) {
	return NewFromConfig(handler, &Config{
		Name:         cfg.Name,
		Addr:         cfg.Addr,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
		Runtime:      cfg.Runtime,
		Logging:      cfg.Logging,
	}, log, additionalOpts...)
}

// NewFromConfig 从配置创建 HTTP 服务器.
//
// 将 Config 字段转换为对应的 WithXxx 选项，然后调用 New.
// additionalOpts 可用于补充 Config 无法表达的选项如自定义中间件等需要运行时对象的配置.
// 如果 TLS 配置无效，返回 (nil, error) 而非 panic.
//
// 示例:
//
//	cfg := &httpserver.Config{
//	    Name:      "api",
//	    Addr:      ":8080",
//	    Profiling: "/debug/pprof",
//	}
//	srv, err := httpserver.NewFromConfig(handler, cfg, log,
//	    httpserver.WithMiddlewares(recovery.HTTPMiddleware(), logging.HTTPMiddleware()),
//	)
func NewFromConfig(handler http.Handler, cfg *Config, log logger.Logger, additionalOpts ...Option) (*Server, error) {
	var opts []Option

	// 日志记录器必需
	opts = append(opts, WithLogger(log))

	// 基本配置
	if cfg.Name != "" {
		opts = append(opts, WithName(cfg.Name))
	}
	if cfg.Addr != "" {
		opts = append(opts, WithAddr(cfg.Addr))
	}

	// 超时配置
	if cfg.ReadTimeout > 0 || cfg.WriteTimeout > 0 || cfg.IdleTimeout > 0 {
		opts = append(opts, WithTimeout(cfg.ReadTimeout, cfg.WriteTimeout, cfg.IdleTimeout))
	}

	// TLS 配置：返回 error 而非 panic
	if cfg.TLS != nil {
		tlsCfg, err := tlsx.NewServerTLSConfig(cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("httpserver: 创建 TLS 配置失败: %w", err)
		}
		opts = append(opts, WithTLS(tlsCfg))
	}

	// Profiling 配置
	if cfg.Profiling != "" {
		opts = append(opts, WithProfiling(cfg.Profiling))
	}

	var middlewares []func(http.Handler) http.Handler
	if cfg.Runtime.Recovery {
		middlewares = append(middlewares, recovery.HTTPMiddleware(recovery.WithLogger(log)))
	}
	if cfg.Logging.Enabled {
		middlewares = append(middlewares, logging.HTTPMiddleware(
			logging.WithLogger(log),
			logging.WithSkipPaths(cfg.Logging.SkipPaths...),
		))
	}
	if len(middlewares) > 0 {
		opts = append(opts, WithMiddlewares(middlewares...))
	}

	// 附加用户自定义选项
	opts = append(opts, additionalOpts...)

	return New(handler, opts...), nil
}
