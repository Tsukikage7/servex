package telegram

import (
	"github.com/Tsukikage7/servex/v2/transport/botserver"
	"github.com/Tsukikage7/servex/v2/transport/httpserver"
)

// Option TelegramBot 选项。
type Option func(*TelegramBot)

// WithWebhookPath 设置 webhook 路径，默认 "/bot/telegram"。
func WithWebhookPath(path string) Option {
	return func(b *TelegramBot) {
		b.webhookPath = path
	}
}

// WithWebhookURL 设置公网 HTTPS Webhook URL，Start 时调用 SetWebhook。
// 若未设置，Start 跳过 SetWebhook，调用者需自行调用。
func WithWebhookURL(url string) Option {
	return func(b *TelegramBot) {
		b.webhookURL = url
	}
}

// WithStateStore 设置对话状态存储。
func WithStateStore(s botserver.StateStore) Option {
	return func(b *TelegramBot) {
		b.store = s
	}
}

// WithHTTPServer 将 webhook 路由注册到现有 httpserver Router。
func WithHTTPServer(r *httpserver.Router) Option {
	return func(b *TelegramBot) {
		b.httpRouter = r
	}
}

// WithErrorHandler 设置 handler 错误处理函数。
func WithErrorHandler(h func(ctx botserver.Context, err error)) Option {
	return func(b *TelegramBot) {
		b.errHandler = h
	}
}
