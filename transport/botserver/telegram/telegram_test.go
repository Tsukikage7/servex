package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/Tsukikage7/servex/v2/transport/botserver"
	"github.com/Tsukikage7/servex/v2/transport/httpserver"
)

// ---- 辅助函数 ----

// newTestUpdate 构造一条带 Message 的测试 Update。
// 若 text 以 "/" 开头，会设置 Entities 令 tgbotapi 识别为命令。
func newTestUpdate(chatID int64, userID int64, text string) *tgbotapi.Update {
	msg := &tgbotapi.Message{
		MessageID: 1,
		Chat:      &tgbotapi.Chat{ID: chatID},
		From:      &tgbotapi.User{ID: userID},
		Text:      text,
	}
	if len(text) > 0 && text[0] == '/' {
		end := len(text)
		for i, ch := range text {
			if ch == ' ' && i > 0 {
				end = i
				break
			}
		}
		msg.Entities = []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: end},
		}
	}
	return &tgbotapi.Update{
		UpdateID: 42,
		Message:  msg,
	}
}

// newTestBot 构造不依赖网络的 TelegramBot（无真实 BotAPI）。
func newTestBot() *TelegramBot {
	b := &TelegramBot{
		router:      botserver.NewRouter(),
		store:       botserver.NewMemoryStateStore(),
		webhookPath: "/bot/telegram",
		errHandler: func(_ botserver.Context, err error) {
		},
	}
	b.router.SetErrorHandler(b.errHandler)
	return b
}

// ---- tgContext 单元测试 ----

func TestTgContext_ChatID_UserID(t *testing.T) {
	store := botserver.NewMemoryStateStore()
	update := newTestUpdate(100, 200, "hello")
	ctx := newTgContext(update, nil, store)

	if got := ctx.ChatID(); got != "100" {
		t.Errorf("ChatID() = %q, want %q", got, "100")
	}
	if got := ctx.UserID(); got != "200" {
		t.Errorf("UserID() = %q, want %q", got, "200")
	}
}

func TestTgContext_Text(t *testing.T) {
	store := botserver.NewMemoryStateStore()
	update := newTestUpdate(1, 1, "some text")
	ctx := newTgContext(update, nil, store)

	if got := ctx.Text(); got != "some text" {
		t.Errorf("Text() = %q, want %q", got, "some text")
	}
}

func TestTgContext_Command_NoCommand(t *testing.T) {
	store := botserver.NewMemoryStateStore()
	update := newTestUpdate(1, 1, "plain message")
	ctx := newTgContext(update, nil, store)

	if got := ctx.Command(); got != "" {
		t.Errorf("Command() = %q, want empty string for non-command", got)
	}
	if got := ctx.Args(); got != nil {
		t.Errorf("Args() = %v, want nil for non-command", got)
	}
}

func TestTgContext_Command_WithCommand(t *testing.T) {
	store := botserver.NewMemoryStateStore()
	update := newTestUpdate(1, 1, "/start")
	ctx := newTgContext(update, nil, store)

	if got := ctx.Command(); got != "start" {
		t.Errorf("Command() = %q, want %q", got, "start")
	}
	if got := ctx.Args(); got != nil {
		t.Errorf("Args() = %v, want nil for command without args", got)
	}
}

func TestTgContext_Command_WithArgs(t *testing.T) {
	store := botserver.NewMemoryStateStore()
	update := newTestUpdate(1, 1, "/echo hello world")
	ctx := newTgContext(update, nil, store)

	if got := ctx.Command(); got != "echo" {
		t.Errorf("Command() = %q, want %q", got, "echo")
	}
	args := ctx.Args()
	if len(args) != 2 || args[0] != "hello" || args[1] != "world" {
		t.Errorf("Args() = %v, want [hello world]", args)
	}
}

func TestTgContext_State_SetState(t *testing.T) {
	store := botserver.NewMemoryStateStore()
	update := newTestUpdate(555, 1, "/start")
	ctx := newTgContext(update, nil, store)

	// 初始状态应为空
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

func TestTgContext_Native(t *testing.T) {
	store := botserver.NewMemoryStateStore()
	update := newTestUpdate(1, 1, "hi")
	ctx := newTgContext(update, nil, store)

	got, ok := ctx.Native().(*tgbotapi.Update)
	if !ok {
		t.Fatalf("Native() type = %T, want *tgbotapi.Update", ctx.Native())
	}
	if got.UpdateID != 42 {
		t.Errorf("Native().UpdateID = %d, want 42", got.UpdateID)
	}
}

func TestTgContext_UserID_NilFrom(t *testing.T) {
	store := botserver.NewMemoryStateStore()
	update := newTestUpdate(1, 0, "hi")
	update.Message.From = nil
	ctx := newTgContext(update, nil, store)

	if got := ctx.UserID(); got != "" {
		t.Errorf("UserID() with nil From = %q, want empty", got)
	}
}

// ---- handleUpdate 集成测试（不需要真实 BotAPI）----

func TestHandleUpdate_NilMessage(t *testing.T) {
	// 若 Message 为 nil，handler 不应被调用
	called := false

	b := newTestBot()
	b.router.Handle("*", func(_ botserver.Context) error {
		called = true
		return nil
	})

	// 构造只有 CallbackQuery 的 Update（Message 为 nil）
	update := &tgbotapi.Update{
		UpdateID:      99,
		CallbackQuery: &tgbotapi.CallbackQuery{ID: "cq1"},
	}
	body, _ := json.Marshal(update)

	req := httptest.NewRequest(http.MethodPost, "/bot/telegram", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	b.handleUpdate(rr, req)

	if called {
		t.Error("handler should not be called when Message is nil")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestHandleUpdate_ValidMessage(t *testing.T) {
	handled := false

	b := newTestBot()
	b.router.Handle("start", func(ctx botserver.Context) error {
		handled = true
		if ctx.ChatID() != "10" {
			t.Errorf("ChatID = %q, want %q", ctx.ChatID(), "10")
		}
		return nil
	})

	update := newTestUpdate(10, 20, "/start")
	body, _ := json.Marshal(update)

	req := httptest.NewRequest(http.MethodPost, "/bot/telegram", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	b.handleUpdate(rr, req)

	if !handled {
		t.Error("handler was not called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestHandleUpdate_BadJSON(t *testing.T) {
	b := newTestBot()

	req := httptest.NewRequest(http.MethodPost, "/bot/telegram", bytes.NewReader([]byte("not-json")))
	rr := httptest.NewRecorder()
	b.handleUpdate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for bad JSON", rr.Code)
	}
}

// ---- TelegramBot Handle/Use 测试 ----

func TestTelegramBot_HandleUse(t *testing.T) {
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

	update := newTestUpdate(1, 1, "/hello")
	ctx := newTgContext(update, nil, b.store)
	_ = b.router.Dispatch(ctx)

	if !mwCalled {
		t.Error("global middleware was not called")
	}
	if !handlerCalled {
		t.Error("handler was not called")
	}
}

// ---- Start 流程测试（不需要网络）----

func TestTelegramBot_Start_NoWebhookURL(t *testing.T) {
	// 未设置 webhookURL 且未设置 client 时，Start 应跳过 SetWebhook 并返回 nil
	b := newTestBot()
	// client 为 nil，若 Start 错误地调用了 SetWebhook 会 panic

	if err := b.Start(context.Background()); err != nil {
		t.Errorf("Start() error = %v, want nil when no webhookURL", err)
	}
}

func TestTelegramBot_Start_RegistersHTTPRoute(t *testing.T) {
	// 验证 httpRouter 非 nil 时，POST 路由被注册，Webhook 请求能被处理
	handled := false

	b := newTestBot()
	b.Handle("ping", func(_ botserver.Context) error {
		handled = true
		return nil
	})

	httpRouter := httpserver.NewRouter()
	b.httpRouter = httpRouter

	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// 通过 httptest.Server 发送请求，验证路由已注册
	srv := httptest.NewServer(httpRouter)
	defer srv.Close()

	update := newTestUpdate(1, 1, "/ping")
	body, _ := json.Marshal(update)

	resp, err := http.Post(srv.URL+"/bot/telegram", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if !handled {
		t.Error("handler was not called via httpRouter")
	}
}
