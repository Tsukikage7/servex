package email

import "github.com/Tsukikage7/servex/v2/observability/logger"

type senderOptions struct {
	host     string
	port     int
	username string
	password string
	fromAddr string
	fromName string
	useTLS   bool
	logger   logger.Logger
}

// Option 邮件发送器配置选项.
type Option func(*senderOptions)

// WithSMTP 设置 SMTP 服务器地址和端口.
func WithSMTP(host string, port int) Option {
	return func(o *senderOptions) { o.host = host; o.port = port }
}

// WithAuth 设置 SMTP 认证用户名和密码.
func WithAuth(username, password string) Option {
	return func(o *senderOptions) { o.username = username; o.password = password }
}

// WithFrom 设置发件人地址和显示名称.
func WithFrom(addr, name string) Option {
	return func(o *senderOptions) { o.fromAddr = addr; o.fromName = name }
}

// WithTLS 设置是否启用 TLS 连接.
func WithTLS(enable bool) Option {
	return func(o *senderOptions) { o.useTLS = enable }
}

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option {
	return func(o *senderOptions) { o.logger = log }
}
