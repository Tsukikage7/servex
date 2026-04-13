package discord

import (
	"github.com/bwmarrin/discordgo"

	"github.com/Tsukikage7/servex/transport/botserver"
)

// Option DiscordBot 选项。
type Option func(*DiscordBot)

// WithStateStore 设置对话状态存储。
func WithStateStore(s botserver.StateStore) Option {
	return func(b *DiscordBot) {
		b.stateStore = s
	}
}

// WithIntents 覆盖 Gateway Intents。
func WithIntents(intents discordgo.Intent) Option {
	return func(b *DiscordBot) {
		b.session.Identify.Intents = intents
	}
}

// WithCommandPrefix 设置消息命令前缀，默认 "/"。
func WithCommandPrefix(prefix string) Option {
	return func(b *DiscordBot) {
		b.prefix = prefix
	}
}

// WithErrorHandler 设置 handler 错误处理函数。
func WithErrorHandler(h func(ctx botserver.Context, err error)) Option {
	return func(b *DiscordBot) {
		b.errHandler = h
	}
}
