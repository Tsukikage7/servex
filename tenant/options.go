package tenant

import (
	"context"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Skipper 判断是否跳过租户解析.
type Skipper func(ctx context.Context, request any) bool

// ErrorHandler 自定义错误处理.
type ErrorHandler func(ctx context.Context, err error) error

// options 中间件配置.
type options struct {
	skipper        Skipper
	tokenExtractor TokenExtractor
	errorHandler   ErrorHandler
	logger         logger.Logger
}

// Option 配置选项函数.
type Option func(*options)

func defaultOptions() *options {
	return &options{}
}

func applyOptions(opts []Option) *options {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithSkipper 设置跳过器.
func WithSkipper(s Skipper) Option {
	return func(o *options) { o.skipper = s }
}

// WithTokenExtractor 设置令牌提取器.
func WithTokenExtractor(e TokenExtractor) Option {
	return func(o *options) { o.tokenExtractor = e }
}

// WithErrorHandler 设置错误处理器.
func WithErrorHandler(h ErrorHandler) Option {
	return func(o *options) { o.errorHandler = h }
}

// WithLogger 设置日志记录器.
func WithLogger(l logger.Logger) Option {
	return func(o *options) { o.logger = logger.WithComponent(l, "Tenant") }
}

// handleError 处理错误.
func handleError(ctx context.Context, err error, o *options) error {
	if o.errorHandler != nil {
		return o.errorHandler(ctx, err)
	}
	return err
}
