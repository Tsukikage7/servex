package httpserver

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Tsukikage7/servex/observability/logger"
	tlsx "github.com/Tsukikage7/servex/transport/tls"
)

// Config HTTP 服务器配置.
type Config struct {
	Name         string        `json:"name" yaml:"name" mapstructure:"name"`
	Addr         string        `json:"addr" yaml:"addr" mapstructure:"addr"`
	ReadTimeout  time.Duration `json:"read_timeout" yaml:"read_timeout" mapstructure:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout" mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `json:"idle_timeout" yaml:"idle_timeout" mapstructure:"idle_timeout"`
	TLS          *tlsx.Config  `json:"tls" yaml:"tls" mapstructure:"tls"`
	Profiling    string        `json:"profiling" yaml:"profiling" mapstructure:"profiling"` // 路径前缀，空字符串表示不启用
}

// NewFromConfig 从配置创建 HTTP 服务器.
//
// 将 Config 字段转换为对应的 WithXxx 选项，然后调用 New.
// additionalOpts 可用于补充 Config 无法表达的选项（如自定义中间件等需要运行时对象的配置）.
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

	// 日志记录器（必需）
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

	// 附加用户自定义选项
	opts = append(opts, additionalOpts...)

	return New(handler, opts...), nil
}
