// Package telegram 提供基于 Webhook 模式的 Telegram Bot 实现。
package telegram

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/Tsukikage7/servex/v2/transport/botserver"
	"github.com/Tsukikage7/servex/v2/transport/httpserver"
)

// TelegramBot Telegram Bot 实现，使用 Webhook 模式。
type TelegramBot struct {
	client      *tgbotapi.BotAPI
	router      *botserver.Router
	store       botserver.StateStore
	webhookPath string
	webhookURL  string
	httpRouter  *httpserver.Router // 可为 nil，用于注册 webhook 路由
	errHandler  func(ctx botserver.Context, err error)
}

// New 创建 TelegramBot。
func New(token string, opts ...Option) (*TelegramBot, error) {
	client, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	b := &TelegramBot{
		client:      client,
		router:      botserver.NewRouter(),
		store:       botserver.NewMemoryStateStore(),
		webhookPath: "/bot/telegram",
		errHandler: func(ctx botserver.Context, err error) {
			log.Printf("component=Telegram 处理器执行失败 chat=%s error=%v", ctx.ChatID(), err)
		},
	}

	for _, opt := range opts {
		opt(b)
	}

	// 将错误处理器同步给 router（router 内部默认有自己的，通过 SetErrorHandler 覆盖）
	b.router.SetErrorHandler(b.errHandler)

	return b, nil
}

// Client 暴露底层 BotAPI，供 notify/telegram 等复用。
func (b *TelegramBot) Client() *tgbotapi.BotAPI {
	return b.client
}

// Handle 注册命令处理器。
func (b *TelegramBot) Handle(pattern string, handler botserver.HandlerFunc, middlewares ...botserver.Middleware) {
	b.router.Handle(pattern, handler, middlewares...)
}

// Use 注册全局中间件。
func (b *TelegramBot) Use(middlewares ...botserver.Middleware) {
	b.router.Use(middlewares...)
}

// Start 设置 Webhook（若配置了 webhookURL），注册 HTTP 路由（若配置了 httpRouter）。
// Webhook 模式下为非阻塞，消息驱动由 httpserver 处理。
func (b *TelegramBot) Start(_ context.Context) error {
	// 1. 若配置了 webhookURL，调用 SetWebhook
	if b.webhookURL != "" {
		wh, err := tgbotapi.NewWebhook(b.webhookURL)
		if err != nil {
			return err
		}
		if _, err = b.client.Request(wh); err != nil {
			return err
		}
		log.Printf("component=Telegram Webhook 已设置 url=%s", b.webhookURL)
	}

	// 2. 若配置了 httpRouter，注册 POST 路由
	if b.httpRouter != nil {
		b.httpRouter.POST(b.webhookPath, http.HandlerFunc(b.handleUpdate))
		log.Printf("component=Telegram Webhook 处理器已注册 method=POST path=%s", b.webhookPath)
	}

	return nil
}

// Stop 删除 Webhook。
func (b *TelegramBot) Stop() error {
	_, err := b.client.Request(tgbotapi.DeleteWebhookConfig{DropPendingUpdates: false})
	return err
}

// handleUpdate 处理来自 Telegram 的 Webhook POST 请求。
func (b *TelegramBot) handleUpdate(w http.ResponseWriter, r *http.Request) {
	var update tgbotapi.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		log.Printf("component=Telegram 更新消息解码失败 error=%v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Message 为 nil 时（如 CallbackQuery）跳过，避免 panic
	if update.Message == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	ctx := newTgContext(&update, b.client, b.store)
	if err := b.router.Dispatch(ctx); err != nil {
		// 错误已由 router 内部 errHandler 处理，此处仍返回 200 避免 Telegram 重试
		log.Printf("component=Telegram 分发消息失败 error=%v", err)
	}

	w.WriteHeader(http.StatusOK)
}
