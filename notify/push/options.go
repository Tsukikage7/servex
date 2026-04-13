package push

import "github.com/Tsukikage7/servex/v2/observability/logger"

type senderOptions struct{ logger logger.Logger }

// Option 推送发送器配置选项.
type Option func(*senderOptions)

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option { return func(o *senderOptions) { o.logger = log } }
