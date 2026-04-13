package bottest_test

import (
	"testing"

	"github.com/Tsukikage7/servex/v2/transport/botserver"
	"github.com/Tsukikage7/servex/v2/transport/botserver/bottest"
)

// TestCommandParsing 验证命令解析逻辑。
func TestCommandParsing(t *testing.T) {
	cases := []struct {
		text        string
		wantCommand string
		wantArgs    []string
	}{
		{"/ping", "ping", nil},
		{"/setname alice", "setname", []string{"alice"}},
		{"/multi a b c", "multi", []string{"a", "b", "c"}},
		{"hello world", "", nil},
		{"plain text", "", nil},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.text, func(t *testing.T) {
			bot, _ := bottest.NewTestBot()

			var gotCommand string
			var gotArgs []string

			bot.Handle(tc.wantCommand, func(ctx botserver.Context) error {
				gotCommand = ctx.Command()
				gotArgs = ctx.Args()
				return nil
			})
			// 通配符捕获非命令
			bot.Handle("*", func(ctx botserver.Context) error {
				gotCommand = ctx.Command()
				gotArgs = ctx.Args()
				return nil
			})

			if err := bot.Dispatch(tc.text); err != nil {
				t.Fatalf("Dispatch error: %v", err)
			}

			if gotCommand != tc.wantCommand {
				t.Errorf("Command: want %q, got %q", tc.wantCommand, gotCommand)
			}

			if len(gotArgs) != len(tc.wantArgs) {
				t.Errorf("Args len: want %d, got %d (%v)", len(tc.wantArgs), len(gotArgs), gotArgs)
				return
			}
			for i, a := range tc.wantArgs {
				if gotArgs[i] != a {
					t.Errorf("Args[%d]: want %q, got %q", i, a, gotArgs[i])
				}
			}
		})
	}
}

// TestReplyRecorded 验证 Reply 被 Recorder 记录。
func TestReplyRecorded(t *testing.T) {
	bot, rec := bottest.NewTestBot()

	bot.Handle("ping", func(ctx botserver.Context) error {
		return ctx.Reply("pong")
	})

	if err := bot.Dispatch("/ping"); err != nil {
		t.Fatalf("Dispatch error: %v", err)
	}

	if len(rec.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(rec.Messages))
	}
	if rec.Messages[0].Text != "pong" {
		t.Errorf("expected text %q, got %q", "pong", rec.Messages[0].Text)
	}
	if rec.Messages[0].ChatID != "test-chat" {
		t.Errorf("expected chatID %q, got %q", "test-chat", rec.Messages[0].ChatID)
	}
}

// TestWithChatIDAndUserID 验证 WithChatID/WithUserID 选项生效。
func TestWithChatIDAndUserID(t *testing.T) {
	bot, rec := bottest.NewTestBot()

	bot.Handle("hello", func(ctx botserver.Context) error {
		if ctx.ChatID() != "custom-chat" {
			t.Errorf("ChatID: want %q, got %q", "custom-chat", ctx.ChatID())
		}
		if ctx.UserID() != "custom-user" {
			t.Errorf("UserID: want %q, got %q", "custom-user", ctx.UserID())
		}
		return ctx.Reply("hi")
	})

	if err := bot.Dispatch("/hello",
		bottest.WithChatID("custom-chat"),
		bottest.WithUserID("custom-user"),
	); err != nil {
		t.Fatalf("Dispatch error: %v", err)
	}

	if len(rec.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(rec.Messages))
	}
	if rec.Messages[0].ChatID != "custom-chat" {
		t.Errorf("Recorder ChatID: want %q, got %q", "custom-chat", rec.Messages[0].ChatID)
	}
}

// TestFullFlow 完整流程：注册 handler → Dispatch → 断言 Recorder。
func TestFullFlow(t *testing.T) {
	bot, rec := bottest.NewTestBot()

	// 全局中间件：在 Text 前附加前缀
	bot.Use(func(next botserver.HandlerFunc) botserver.HandlerFunc {
		return func(ctx botserver.Context) error {
			return next(ctx)
		}
	})

	bot.Handle("greet", func(ctx botserver.Context) error {
		args := ctx.Args()
		name := "World"
		if len(args) > 0 {
			name = args[0]
		}
		return ctx.Reply("Hello, " + name + "!")
	})

	bot.Handle("echo", func(ctx botserver.Context) error {
		return ctx.Reply(ctx.Text())
	})

	// 第一条消息
	if err := bot.Dispatch("/greet Alice", bottest.WithChatID("room-1")); err != nil {
		t.Fatalf("Dispatch greet error: %v", err)
	}
	// 第二条消息
	if err := bot.Dispatch("/echo test message", bottest.WithChatID("room-2")); err != nil {
		t.Fatalf("Dispatch echo error: %v", err)
	}

	if len(rec.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(rec.Messages))
	}

	if rec.Messages[0].ChatID != "room-1" || rec.Messages[0].Text != "Hello, Alice!" {
		t.Errorf("msg[0]: got {%q, %q}", rec.Messages[0].ChatID, rec.Messages[0].Text)
	}
	if rec.Messages[1].ChatID != "room-2" || rec.Messages[1].Text != "/echo test message" {
		t.Errorf("msg[1]: got {%q, %q}", rec.Messages[1].ChatID, rec.Messages[1].Text)
	}
}

// TestStateStorePersistence 验证 State/SetState 在同一 chatID 下持久化。
func TestStateStorePersistence(t *testing.T) {
	bot, _ := bottest.NewTestBot()

	bot.Handle("setState", func(ctx botserver.Context) error {
		ctx.SetState("active")
		return nil
	})

	bot.Handle("getState", func(ctx botserver.Context) error {
		if ctx.State() != "active" {
			t.Errorf("State: want %q, got %q", "active", ctx.State())
		}
		return nil
	})

	chatID := bottest.WithChatID("stateful-chat")

	if err := bot.Dispatch("/setState", chatID); err != nil {
		t.Fatalf("setState error: %v", err)
	}
	if err := bot.Dispatch("/getState", chatID); err != nil {
		t.Fatalf("getState error: %v", err)
	}
}
