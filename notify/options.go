package notify

import "github.com/Tsukikage7/servex/v2/observability/logger"

type dispatcherOptions struct {
	logger         logger.Logger
	templateEngine TemplateEngine
	defaultChannel Channel
}

// Option 分发器配置选项.
type Option func(*dispatcherOptions)

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option {
	return func(o *dispatcherOptions) { o.logger = log }
}

// WithTemplateEngine 设置模板渲染引擎.
func WithTemplateEngine(eng TemplateEngine) Option {
	return func(o *dispatcherOptions) { o.templateEngine = eng }
}

// WithDefaultChannel 设置默认通知渠道.
func WithDefaultChannel(ch Channel) Option {
	return func(o *dispatcherOptions) { o.defaultChannel = ch }
}
