package timeout

import (
	"time"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Option 配置选项函数.
type Option func(*options)

// options 超时配置.
type options struct {
	timeout   time.Duration
	logger    logger.Logger
	onTimeout func(ctx any, duration time.Duration) // 超时回调
}

// defaultOptions 返回默认配置.
func defaultOptions(timeout time.Duration) *options {
	return &options{
		timeout: timeout,
	}
}

// applyOptions 应用配置选项.
func applyOptions(timeout time.Duration, opts []Option) *options {
	o := defaultOptions(timeout)
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithLogger 设置日志记录器.
// 设置后，超时事件会被记录到日志.
func WithLogger(log logger.Logger) Option {
	return func(o *options) {
		o.logger = logger.WithComponent(log, "Timeout")
	}
}

// WithOnTimeout 设置超时回调函数.
// 当请求超时时会调用此函数，ctx 参数类型取决于中间件类型：
//   - Endpoint: context.Context
//   - HTTP: *http.Request
//   - gRPC: context.Context
func WithOnTimeout(fn func(ctx any, duration time.Duration)) Option {
	return func(o *options) {
		o.onTimeout = fn
	}
}
