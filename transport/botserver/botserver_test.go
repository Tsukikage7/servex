package botserver

import (
	"errors"
	"strings"
	"testing"
)

// mockCtx 测试用 Context 实现。
type mockCtx struct {
	chatID  string
	userID  string
	text    string
	command string
	args    []string
	state   string
	replies []string
}

func (m *mockCtx) ChatID() string    { return m.chatID }
func (m *mockCtx) UserID() string    { return m.userID }
func (m *mockCtx) Text() string      { return m.text }
func (m *mockCtx) Command() string   { return m.command }
func (m *mockCtx) Args() []string    { return m.args }
func (m *mockCtx) State() string     { return m.state }
func (m *mockCtx) SetState(s string) { m.state = s }
func (m *mockCtx) Reply(text string, _ ...ReplyOption) error {
	m.replies = append(m.replies, text)
	return nil
}
func (m *mockCtx) Native() any { return nil }

func newMockCtx(command string) *mockCtx {
	return &mockCtx{chatID: "chat1", userID: "user1", command: command}
}

// TestDispatch_ExactMatch 验证精确命令匹配。
func TestDispatch_ExactMatch(t *testing.T) {
	r := newRouter()
	called := false
	r.Handle("start", func(ctx Context) error {
		called = true
		return nil
	})

	if err := r.Dispatch(newMockCtx("start")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler not called")
	}
}

// TestDispatch_WildcardFallback 验证通配符匹配精确不命中时。
func TestDispatch_WildcardFallback(t *testing.T) {
	r := newRouter()
	var got string
	r.Handle("start", func(ctx Context) error {
		got = "start"
		return nil
	})
	r.Handle("*", func(ctx Context) error {
		got = "wildcard"
		return nil
	})

	if err := r.Dispatch(newMockCtx("unknown")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "wildcard" {
		t.Fatalf("expected wildcard, got %q", got)
	}
}

// TestDispatch_ExactPriorityOverWildcard 验证精确匹配优先于通配符。
func TestDispatch_ExactPriorityOverWildcard(t *testing.T) {
	r := newRouter()
	var got string
	r.Handle("*", func(ctx Context) error {
		got = "wildcard"
		return nil
	})
	r.Handle("start", func(ctx Context) error {
		got = "start"
		return nil
	})

	if err := r.Dispatch(newMockCtx("start")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "start" {
		t.Fatalf("expected start, got %q", got)
	}
}

// TestDispatch_NoMatch 验证无匹配时静默忽略。
func TestDispatch_NoMatch(t *testing.T) {
	r := newRouter()
	r.Handle("start", func(ctx Context) error { return nil })
	if err := r.Dispatch(newMockCtx("unknown")); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// TestDispatch_GlobalMiddlewareOrder 验证全局中间件执行顺序先注册先执行。
func TestDispatch_GlobalMiddlewareOrder(t *testing.T) {
	r := newRouter()
	var order []string

	mw := func(name string) Middleware {
		return func(next HandlerFunc) HandlerFunc {
			return func(ctx Context) error {
				order = append(order, name)
				return next(ctx)
			}
		}
	}

	r.Use(mw("mw1"), mw("mw2"))
	r.Handle("start", func(ctx Context) error {
		order = append(order, "handler")
		return nil
	})

	if err := r.Dispatch(newMockCtx("start")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"mw1", "mw2", "handler"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("expected order %v, got %v", want, order)
	}
}

// TestDispatch_RouteLevelMiddlewareOrder 验证路由级中间件执行顺序在全局中间件之后。
func TestDispatch_RouteLevelMiddlewareOrder(t *testing.T) {
	r := newRouter()
	var order []string

	mw := func(name string) Middleware {
		return func(next HandlerFunc) HandlerFunc {
			return func(ctx Context) error {
				order = append(order, name)
				return next(ctx)
			}
		}
	}

	r.Use(mw("global"))
	r.Handle("start", func(ctx Context) error {
		order = append(order, "handler")
		return nil
	}, mw("route"))

	if err := r.Dispatch(newMockCtx("start")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"global", "route", "handler"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("expected order %v, got %v", want, order)
	}
}

// TestDispatch_ErrHandlerCalled 验证 handler 返回 error 时 errHandler 被调用。
func TestDispatch_ErrHandlerCalled(t *testing.T) {
	r := newRouter()
	errHandlerCalled := false
	handlerErr := errors.New("handler error")

	r.errHandler = func(ctx Context, err error) {
		if errors.Is(err, handlerErr) {
			errHandlerCalled = true
		}
	}

	r.Handle("start", func(ctx Context) error {
		return handlerErr
	})

	err := r.Dispatch(newMockCtx("start"))
	if !errors.Is(err, handlerErr) {
		t.Fatalf("expected handler error, got %v", err)
	}
	if !errHandlerCalled {
		t.Fatal("errHandler was not called")
	}
}
