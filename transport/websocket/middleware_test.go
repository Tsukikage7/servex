package websocket

import (
	"testing"

	"github.com/Tsukikage7/servex/v2/testx"
)

func TestLoggingMiddleware(t *testing.T) {
	log := testx.NopLogger()
	var called bool
	inner := func(client Client, msg *Message) {
		called = true
	}

	handler := LoggingMiddleware(log)(inner)
	c := newMockClient("c1")
	handler(c, &Message{Type: TextMessage, Data: []byte("hello")})

	if !called {
		t.Error("inner handler should have been called")
	}
}

func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	log := testx.NopLogger()
	var called bool
	inner := func(client Client, msg *Message) {
		called = true
	}

	handler := RecoveryMiddleware(log)(inner)
	c := newMockClient("c1")
	handler(c, &Message{Type: TextMessage, Data: []byte("ok")})

	if !called {
		t.Error("inner handler should have been called")
	}
}

func TestRecoveryMiddleware_WithPanic(t *testing.T) {
	log := testx.NopLogger()
	inner := func(client Client, msg *Message) {
		panic("test panic")
	}

	handler := RecoveryMiddleware(log)(inner)
	c := newMockClient("c1")

	// Should not propagate panic.
	handler(c, &Message{Type: TextMessage, Data: []byte("boom")})
}

func TestRateLimitMiddleware(t *testing.T) {
	var count int
	inner := func(client Client, msg *Message) {
		count++
	}

	handler := RateLimitMiddleware(2, 1<<62)(inner) // very long window
	c := newMockClient("c1")

	handler(c, &Message{})
	handler(c, &Message{})
	handler(c, &Message{}) // should be rate-limited

	if count != 2 {
		t.Errorf("expected 2 calls, got %d", count)
	}
}

func TestMessageSizeMiddleware(t *testing.T) {
	var called bool
	inner := func(client Client, msg *Message) {
		called = true
	}

	handler := MessageSizeMiddleware(10)(inner)
	c := newMockClient("c1")

	// Small message passes.
	handler(c, &Message{Data: []byte("ok")})
	if !called {
		t.Error("small message should pass through")
	}

	// Large message dropped.
	called = false
	handler(c, &Message{Data: make([]byte, 100)})
	if called {
		t.Error("large message should be dropped")
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	var innerCalled bool
	inner := func(client Client, msg *Message) {
		innerCalled = true
	}

	validator := func(token string) bool { return token == "valid-token" }
	handler := AuthMiddleware(validator)(inner)

	c := newMockClient("c1")

	// First message: auth with valid token.
	handler(c, &Message{Type: TextMessage, Data: []byte("valid-token")})

	if _, ok := c.Metadata()["authenticated"]; !ok {
		t.Error("client should be authenticated")
	}
	if len(c.sent) == 0 {
		t.Error("should have sent auth ok response")
	}

	// Subsequent message: should reach inner handler.
	handler(c, &Message{Type: TextMessage, Data: []byte("hello")})
	if !innerCalled {
		t.Error("inner handler should be called for authenticated client")
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	inner := func(client Client, msg *Message) {
		t.Error("inner handler should not be called")
	}

	validator := func(token string) bool { return false }
	handler := AuthMiddleware(validator)(inner)

	c := newMockClient("c1")
	handler(c, &Message{Type: TextMessage, Data: []byte("bad-token")})

	if c.closed != true {
		t.Error("client should be closed on auth failure")
	}
}

func TestMiddlewareChain(t *testing.T) {
	log := testx.NopLogger()
	var order []string

	inner := func(client Client, msg *Message) {
		order = append(order, "handler")
	}

	// Build chain: Recovery -> Logging -> handler
	h := RecoveryMiddleware(log)(LoggingMiddleware(log)(inner))
	c := newMockClient("c1")
	h(c, &Message{Type: TextMessage, Data: []byte("test")})

	if len(order) != 1 || order[0] != "handler" {
		t.Errorf("expected handler called, got %v", order)
	}
}

// --- Upgrader ---

func TestNewUpgraderNilConfig(t *testing.T) {
	u := NewUpgrader(nil)
	if u == nil {
		t.Fatal("NewUpgrader with nil config should not return nil")
	}
	if u.config.ReadBufferSize != 1024 {
		t.Errorf("expected default ReadBufferSize 1024, got %d", u.config.ReadBufferSize)
	}
}

func TestNewUpgraderCustomConfig(t *testing.T) {
	cfg := &Config{
		ReadBufferSize:  2048,
		WriteBufferSize: 4096,
		CheckOrigin:     func(origin string) bool { return origin == "ok" },
	}
	u := NewUpgrader(cfg)
	if u.config.ReadBufferSize != 2048 {
		t.Errorf("ReadBufferSize = %d, want 2048", u.config.ReadBufferSize)
	}
}
