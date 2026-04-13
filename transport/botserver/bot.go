// Package botserver 提供平台无关的 Bot 接口、路由器与状态存储。
package botserver

import "context"

// Context 每条消息/命令的处理上下文。
type Context interface {
	ChatID() string
	UserID() string
	Text() string
	// Command 返回命令名，如 "/start" -> "start"，非命令返回 ""。
	Command() string
	// Args 返回命令参数，非命令返回 nil。
	Args() []string

	State() string
	SetState(state string)

	Reply(text string, opts ...ReplyOption) error
	// Native 返回平台原始对象。
	Native() any
}

// ReplyOption Reply 选项（预留扩展点）。
type ReplyOption func(*replyOptions)

type replyOptions struct{}

// HandlerFunc 处理函数。
type HandlerFunc func(ctx Context) error

// Middleware 中间件。
type Middleware func(next HandlerFunc) HandlerFunc

// Bot 平台无关接口。
type Bot interface {
	Handle(pattern string, handler HandlerFunc, middlewares ...Middleware)
	Use(middlewares ...Middleware)
	Start(ctx context.Context) error
	Stop() error
}
