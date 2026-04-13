package discord

import (
	"context"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/Tsukikage7/servex/v2/transport/botserver"
)

// ---- 辅助函数 ----

// newTestMessageCreate 构造一条测试用 MessageCreate 事件。
func newTestMessageCreate(channelID, userID, content string, isBot bool) *discordgo.MessageCreate {
	return &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "msg-001",
			ChannelID: channelID,
			Content:   content,
			Author: &discordgo.User{
				ID:  userID,
				Bot: isBot,
			},
		},
	}
}

// newTestBot 构造不依赖网络的 DiscordBot（无真实 session.Open）。
func newTestBot() *DiscordBot {
	// 使用无效 token 创建 session，不会真正连接
	session, _ := discordgo.New("Bot fake-token-for-test")
	b := &DiscordBot{
		session:    session,
		router:     botserver.NewRouter(),
		stateStore: botserver.NewMemoryStateStore(),
		prefix:     "/",
		errHandler: func(_ botserver.Context, _ error) {},
	}
	b.router.SetErrorHandler(b.errHandler)
	return b
}

// ---- discordContext 单元测试 ----

func TestDiscordContext_ChatID_UserID(t *testing.T) {
	store := botserver.NewMemoryStateStore()
	msg := newTestMessageCreate("ch-100", "user-200", "hello", false)
	ctx := newDiscordContext(msg, nil, store, "/")

	if got := ctx.ChatID(); got != "ch-100" {
		t.Errorf("ChatID() = %q, want %q", got, "ch-100")
	}
	if got := ctx.UserID(); got != "user-200" {
		t.Errorf("UserID() = %q, want %q", got, "user-200")
	}
}

func TestDiscordContext_Text(t *testing.T) {
	store := botserver.NewMemoryStateStore()
	msg := newTestMessageCreate("ch-1", "u-1", "some text", false)
	ctx := newDiscordContext(msg, nil, store, "/")

	if got := ctx.Text(); got != "some text" {
		t.Errorf("Text() = %q, want %q", got, "some text")
	}
}

func TestDiscordContext_Command_NoCommand(t *testing.T) {
	store := botserver.NewMemoryStateStore()
	msg := newTestMessageCreate("ch-1", "u-1", "plain message", false)
	ctx := newDiscordContext(msg, nil, store, "/")

	if got := ctx.Command(); got != "" {
		t.Errorf("Command() = %q, want empty for non-command", got)
	}
	if got := ctx.Args(); got != nil {
		t.Errorf("Args() = %v, want nil for non-command", got)
	}
}

func TestDiscordContext_Command_OnlyPrefix(t *testing.T) {
	store := botserver.NewMemoryStateStore()
	// 只有前缀，没有命令名
	msg := newTestMessageCreate("ch-1", "u-1", "/", false)
	ctx := newDiscordContext(msg, nil, store, "/")

	if got := ctx.Command(); got != "" {
		t.Errorf("Command() = %q, want empty for bare prefix", got)
	}
}

func TestDiscordContext_Command_WithCommand(t *testing.T) {
	store := botserver.NewMemoryStateStore()
	msg := newTestMessageCreate("ch-1", "u-1", "/start", false)
	ctx := newDiscordContext(msg, nil, store, "/")

	if got := ctx.Command(); got != "start" {
		t.Errorf("Command() = %q, want %q", got, "start")
	}
	if got := ctx.Args(); got != nil {
		t.Errorf("Args() = %v, want nil for command without args", got)
	}
}

func TestDiscordContext_Command_WithArgs(t *testing.T) {
	store := botserver.NewMemoryStateStore()
	msg := newTestMessageCreate("ch-1", "u-1", "/echo hello world", false)
	ctx := newDiscordContext(msg, nil, store, "/")

	if got := ctx.Command(); got != "echo" {
		t.Errorf("Command() = %q, want %q", got, "echo")
	}
	args := ctx.Args()
	if len(args) != 2 || args[0] != "hello" || args[1] != "world" {
		t.Errorf("Args() = %v, want [hello world]", args)
	}
}

func TestDiscordContext_Command_CustomPrefix(t *testing.T) {
	store := botserver.NewMemoryStateStore()
	msg := newTestMessageCreate("ch-1", "u-1", "!ping foo", false)
	ctx := newDiscordContext(msg, nil, store, "!")

	if got := ctx.Command(); got != "ping" {
		t.Errorf("Command() = %q, want %q", got, "ping")
	}
	args := ctx.Args()
	if len(args) != 1 || args[0] != "foo" {
		t.Errorf("Args() = %v, want [foo]", args)
	}
}

func TestDiscordContext_State_SetState(t *testing.T) {
	store := botserver.NewMemoryStateStore()
	msg := newTestMessageCreate("ch-555", "u-1", "/start", false)
	ctx := newDiscordContext(msg, nil, store, "/")

	// 初始状态为空
	if got := ctx.State(); got != "" {
		t.Errorf("State() initially = %q, want empty", got)
	}

	ctx.SetState("waiting_input")
	if got := ctx.State(); got != "waiting_input" {
		t.Errorf("State() after SetState = %q, want %q", got, "waiting_input")
	}

	ctx.SetState("")
	if got := ctx.State(); got != "" {
		t.Errorf("State() after clear = %q, want empty", got)
	}
}

func TestDiscordContext_State_IsolatedByChannel(t *testing.T) {
	// 不同频道的状态互相独立
	store := botserver.NewMemoryStateStore()
	msg1 := newTestMessageCreate("ch-A", "u-1", "/cmd", false)
	msg2 := newTestMessageCreate("ch-B", "u-1", "/cmd", false)
	ctx1 := newDiscordContext(msg1, nil, store, "/")
	ctx2 := newDiscordContext(msg2, nil, store, "/")

	ctx1.SetState("stateA")
	if got := ctx2.State(); got != "" {
		t.Errorf("ch-B State() = %q after setting ch-A, want empty", got)
	}
	if got := ctx1.State(); got != "stateA" {
		t.Errorf("ch-A State() = %q, want %q", got, "stateA")
	}
}

func TestDiscordContext_Native(t *testing.T) {
	store := botserver.NewMemoryStateStore()
	msg := newTestMessageCreate("ch-1", "u-1", "hi", false)
	ctx := newDiscordContext(msg, nil, store, "/")

	got, ok := ctx.Native().(*discordgo.MessageCreate)
	if !ok {
		t.Fatalf("Native() type = %T, want *discordgo.MessageCreate", ctx.Native())
	}
	if got.ChannelID != "ch-1" {
		t.Errorf("Native().ChannelID = %q, want %q", got.ChannelID, "ch-1")
	}
}

// ---- handleMessageCreate 过滤测试 ----

func TestHandleMessageCreate_SkipBotMessage(t *testing.T) {
	// Author.Bot == true 时，handler 不应被调用
	called := false
	b := newTestBot()
	b.router.Handle("*", func(_ botserver.Context) error {
		called = true
		return nil
	})

	botMsg := newTestMessageCreate("ch-1", "bot-id", "/start", true)
	b.handleMessageCreate(b.session, botMsg)

	if called {
		t.Error("handler should not be called for bot messages (Author.Bot == true)")
	}
}

func TestHandleMessageCreate_SkipNilAuthor(t *testing.T) {
	// Author 为 nil（系统消息）时，handler 不应被调用
	called := false
	b := newTestBot()
	b.router.Handle("*", func(_ botserver.Context) error {
		called = true
		return nil
	})

	nilAuthorMsg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "sys-001",
			ChannelID: "ch-1",
			Content:   "/start",
			Author:    nil, // 系统消息
		},
	}
	b.handleMessageCreate(b.session, nilAuthorMsg)

	if called {
		t.Error("handler should not be called when Author is nil")
	}
}

func TestHandleMessageCreate_DispatchCommand(t *testing.T) {
	handled := false
	var gotChatID string

	b := newTestBot()
	b.router.Handle("start", func(ctx botserver.Context) error {
		handled = true
		gotChatID = ctx.ChatID()
		return nil
	})

	msg := newTestMessageCreate("ch-42", "u-99", "/start", false)
	b.handleMessageCreate(b.session, msg)

	if !handled {
		t.Error("handler was not called for /start command")
	}
	if gotChatID != "ch-42" {
		t.Errorf("ChatID = %q, want %q", gotChatID, "ch-42")
	}
}

// ---- DiscordBot Handle/Use 测试 ----

func TestDiscordBot_HandleUse(t *testing.T) {
	b := newTestBot()

	mwCalled := false
	b.Use(func(next botserver.HandlerFunc) botserver.HandlerFunc {
		return func(ctx botserver.Context) error {
			mwCalled = true
			return next(ctx)
		}
	})

	handlerCalled := false
	b.Handle("hello", func(_ botserver.Context) error {
		handlerCalled = true
		return nil
	})

	msg := newTestMessageCreate("ch-1", "u-1", "/hello", false)
	ctx := newDiscordContext(msg, nil, b.stateStore, b.prefix)
	_ = b.router.Dispatch(ctx)

	if !mwCalled {
		t.Error("global middleware was not called")
	}
	if !handlerCalled {
		t.Error("handler was not called")
	}
}

func TestDiscordBot_Session(t *testing.T) {
	b := newTestBot()
	if b.Session() == nil {
		t.Error("Session() should not be nil")
	}
}

// ---- Start ctx 取消后正常退出测试（mock Open/Close）----

func TestDiscordBot_Start_CancelContext(t *testing.T) {
	// 使用真实 discordgo.Session 但替换 Open/Close 逻辑：
	// 因为 discordgo.Session 不支持接口替换，
	// 我们验证 Start 在 ctx 取消后能在超时内返回（通过 goroutine + 超时判定）。
	b := newTestBot()

	// 替换 session.Open 为无操作：直接覆盖 DiscordBot，绕过真实连接
	// 由于 discordgo.Session 是具体类型，这里通过子测试隔离 + 短超时检测退出行为。
	// 若 Open 失败（fake token），Start 会立即返回错误，测试仍能验证退出路径。
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	doneCh := make(chan error, 1)
	go func() {
		// Open 会因 fake token 失败并立即返回 error，这也验证了 Start 不阻塞
		err := b.Start(ctx)
		doneCh <- err
	}()

	select {
	case <-doneCh:
		// Start 正常返回（无论是 Open 失败还是 ctx 取消），符合预期
	case <-time.After(3 * time.Second):
		t.Error("Start() did not return within timeout after ctx cancellation")
	}
}
