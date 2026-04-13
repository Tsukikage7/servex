// Package discord 提供基于 Gateway（WebSocket）模式的 Discord Bot 实现。
package discord

import (
	"context"
	"log"

	"github.com/bwmarrin/discordgo"

	"github.com/Tsukikage7/servex/v2/transport/botserver"
)

// DiscordBot Discord Bot 实现，使用 Gateway（WebSocket）模式。
type DiscordBot struct {
	session    *discordgo.Session
	router     *botserver.Router
	stateStore botserver.StateStore
	errHandler func(ctx botserver.Context, err error)
	prefix     string // 消息命令前缀，默认 "/"
}

// New 创建 DiscordBot。
func New(token string, opts ...Option) (*DiscordBot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	b := &DiscordBot{
		session:    session,
		router:     botserver.NewRouter(),
		stateStore: botserver.NewMemoryStateStore(),
		prefix:     "/",
		errHandler: func(ctx botserver.Context, err error) {
			log.Printf("discord: handler error [chat=%s]: %v", ctx.ChatID(), err)
		},
	}

	// 默认 Intents：频道消息 + 私信 + 消息内容
	b.session.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	for _, opt := range opts {
		opt(b)
	}

	// 将错误处理器同步给 router
	b.router.SetErrorHandler(b.errHandler)

	return b, nil
}

// Session 暴露底层 discordgo.Session，供 notify/discord 等复用。
func (b *DiscordBot) Session() *discordgo.Session {
	return b.session
}

// Handle 注册命令处理器。
func (b *DiscordBot) Handle(pattern string, handler botserver.HandlerFunc, middlewares ...botserver.Middleware) {
	b.router.Handle(pattern, handler, middlewares...)
}

// Use 注册全局中间件。
func (b *DiscordBot) Use(middlewares ...botserver.Middleware) {
	b.router.Use(middlewares...)
}

// Start 建立 Gateway 连接，注册消息事件 handler，阻塞直到 ctx 取消。
func (b *DiscordBot) Start(ctx context.Context) error {
	// 注册 MessageCreate 事件 handler
	b.session.AddHandler(b.handleMessageCreate)

	// 建立 WebSocket 连接
	if err := b.session.Open(); err != nil {
		return err
	}
	log.Printf("discord: gateway connected")

	// 阻塞直到 ctx 取消
	<-ctx.Done()

	// ctx 取消后关闭连接
	return b.session.Close()
}

// Stop 关闭 Gateway 连接。
func (b *DiscordBot) Stop() error {
	return b.session.Close()
}

// handleMessageCreate 处理 Discord MessageCreate 事件。
func (b *DiscordBot) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// 系统消息 Author 为 nil，跳过
	if m.Author == nil {
		return
	}
	// 跳过 Bot 自身发送的消息
	if m.Author.Bot {
		return
	}

	ctx := newDiscordContext(m, s, b.stateStore, b.prefix)
	if err := b.router.Dispatch(ctx); err != nil {
		log.Printf("discord: dispatch error: %v", err)
	}
}
