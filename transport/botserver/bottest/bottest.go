// Package bottest 提供用于测试 botserver handler 的工具。
package bottest

import (
	"context"
	"strings"

	"github.com/Tsukikage7/servex/transport/botserver"
)

// RecordedMessage 记录一条 Reply 调用。
type RecordedMessage struct {
	ChatID string
	Text   string
}

// Recorder 记录所有 Reply 调用。
type Recorder struct {
	Messages []RecordedMessage
}

// TestBot 测试用 Bot，实现 botserver.Bot 接口。
// 内部使用 botserver.Router 驱动 handler 分发。
type TestBot struct {
	router   *botserver.Router
	recorder *Recorder
	store    botserver.StateStore
}

// NewTestBot 创建测试 Bot 和 Recorder。
func NewTestBot() (*TestBot, *Recorder) {
	rec := &Recorder{}
	bot := &TestBot{
		router:   botserver.NewRouter(),
		recorder: rec,
		store:    botserver.NewMemoryStateStore(),
	}
	return bot, rec
}

// Handle 注册命令处理器（委托给内部 Router）。
func (b *TestBot) Handle(pattern string, handler botserver.HandlerFunc, middlewares ...botserver.Middleware) {
	b.router.Handle(pattern, handler, middlewares...)
}

// Use 注册全局中间件（委托给内部 Router）。
func (b *TestBot) Use(middlewares ...botserver.Middleware) {
	b.router.Use(middlewares...)
}

// Start 空实现（测试中不需要）。
func (b *TestBot) Start(_ context.Context) error { return nil }

// Stop 空实现。
func (b *TestBot) Stop() error { return nil }

// dispatchOptions Dispatch 内部选项。
type dispatchOptions struct {
	chatID string
	userID string
}

// DispatchOption Dispatch 选项函数。
type DispatchOption func(*dispatchOptions)

// WithChatID 设置会话 ID（默认 "test-chat"）。
func WithChatID(id string) DispatchOption {
	return func(o *dispatchOptions) {
		o.chatID = id
	}
}

// WithUserID 设置用户 ID（默认 "test-user"）。
func WithUserID(id string) DispatchOption {
	return func(o *dispatchOptions) {
		o.userID = id
	}
}

// Dispatch 模拟一条入站消息/命令，触发 handler 执行。
// text 示例："/ping"、"/setname Alice"、"hello world"。
func (b *TestBot) Dispatch(text string, opts ...DispatchOption) error {
	o := &dispatchOptions{
		chatID: "test-chat",
		userID: "test-user",
	}
	for _, opt := range opts {
		opt(o)
	}

	ctx := &testContext{
		chatID:   o.chatID,
		userID:   o.userID,
		text:     text,
		store:    b.store,
		recorder: b.recorder,
	}
	return b.router.Dispatch(ctx)
}

// testContext 实现 botserver.Context 接口，用于测试。
type testContext struct {
	chatID   string
	userID   string
	text     string
	store    botserver.StateStore
	recorder *Recorder
}

// ChatID 返回会话 ID。
func (c *testContext) ChatID() string { return c.chatID }

// UserID 返回用户 ID。
func (c *testContext) UserID() string { return c.userID }

// Text 返回原始消息文本。
func (c *testContext) Text() string { return c.text }

// Command 解析命令名。
// 若 text 以 "/" 开头，返回第一个词去掉 "/"；否则返回 ""。
func (c *testContext) Command() string {
	if !strings.HasPrefix(c.text, "/") {
		return ""
	}
	parts := strings.Fields(c.text)
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimPrefix(parts[0], "/")
}

// Args 解析命令参数。
// "/ping a b" -> ["a", "b"]；非命令或无参数返回 nil。
func (c *testContext) Args() []string {
	if !strings.HasPrefix(c.text, "/") {
		return nil
	}
	parts := strings.Fields(c.text)
	if len(parts) <= 1 {
		return nil
	}
	return parts[1:]
}

// State 获取当前会话状态。
func (c *testContext) State() string {
	s, _ := c.store.Get(c.chatID)
	return s
}

// SetState 设置当前会话状态。
func (c *testContext) SetState(state string) {
	_ = c.store.Set(c.chatID, state)
}

// Reply 将回复追加到 Recorder。
func (c *testContext) Reply(text string, _ ...botserver.ReplyOption) error {
	c.recorder.Messages = append(c.recorder.Messages, RecordedMessage{
		ChatID: c.chatID,
		Text:   text,
	})
	return nil
}

// Native 返回 nil（测试环境无平台原始对象）。
func (c *testContext) Native() any { return nil }
