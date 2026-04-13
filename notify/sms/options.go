package sms

import "github.com/Tsukikage7/servex/v2/observability/logger"

type senderOptions struct {
	signName string
	logger   logger.Logger
}

// Option 短信发送器配置选项.
type Option func(*senderOptions)

// WithSignName 设置短信签名.
func WithSignName(name string) Option {
	return func(o *senderOptions) { o.signName = name }
}

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option {
	return func(o *senderOptions) { o.logger = log }
}
