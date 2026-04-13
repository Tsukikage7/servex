package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Tsukikage7/servex/v2/domain/cqrs"
	"github.com/Tsukikage7/servex/v2/testx"
)

// mockCommandHandler is a test helper for CommandHandler.
type mockCommandHandler[C, R any] struct {
	fn func(ctx context.Context, cmd C) (C, R, error)
}

func (m *mockCommandHandler[C, R]) Handle(ctx context.Context, cmd C) (C, R, error) {
	return m.fn(ctx, cmd)
}

// mockQueryHandler is a test helper for QueryHandler.
type mockQueryHandler[Q, R any] struct {
	fn func(ctx context.Context, query Q) (R, error)
}

func (m *mockQueryHandler[Q, R]) Handle(ctx context.Context, query Q) (R, error) {
	return m.fn(ctx, query)
}

func TestCommandLogging_Success(t *testing.T) {
	log := testx.NopLogger()
	inner := &mockCommandHandler[string, string]{
		fn: func(ctx context.Context, cmd string) (string, string, error) {
			return cmd, "result", nil
		},
	}

	mw := CommandLogging[string, string](log, "TestCommand")
	handler := mw(inner)

	cmd, resp, err := handler.Handle(t.Context(), "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "input" || resp != "result" {
		t.Fatalf("got cmd=%q resp=%q", cmd, resp)
	}
}

func TestCommandLogging_Error(t *testing.T) {
	log := testx.NopLogger()
	errTest := errors.New("command failed")
	inner := &mockCommandHandler[string, string]{
		fn: func(ctx context.Context, cmd string) (string, string, error) {
			return cmd, "", errTest
		},
	}

	mw := CommandLogging[string, string](log, "FailCommand")
	handler := mw(inner)

	_, _, err := handler.Handle(t.Context(), "input")
	if !errors.Is(err, errTest) {
		t.Fatalf("expected errTest, got %v", err)
	}
}

func TestQueryLogging_Success(t *testing.T) {
	log := testx.NopLogger()
	inner := &mockQueryHandler[string, int]{
		fn: func(ctx context.Context, query string) (int, error) {
			return 42, nil
		},
	}

	mw := QueryLogging[string, int](log, "TestQuery")
	handler := mw(inner)

	resp, err := handler.Handle(t.Context(), "q")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != 42 {
		t.Fatalf("got %d, want 42", resp)
	}
}

func TestQueryLogging_Error(t *testing.T) {
	log := testx.NopLogger()
	errTest := errors.New("query failed")
	inner := &mockQueryHandler[string, int]{
		fn: func(ctx context.Context, query string) (int, error) {
			return 0, errTest
		},
	}

	mw := QueryLogging[string, int](log, "FailQuery")
	handler := mw(inner)

	_, err := handler.Handle(t.Context(), "q")
	if !errors.Is(err, errTest) {
		t.Fatalf("expected errTest, got %v", err)
	}
}

func TestCommandMetrics_Success(t *testing.T) {
	reg := prometheus.NewRegistry()
	inner := &mockCommandHandler[string, string]{
		fn: func(ctx context.Context, cmd string) (string, string, error) {
			return cmd, "ok", nil
		},
	}

	mw := CommandMetrics[string, string]("test_cmd", reg)
	handler := mw(inner)

	_, _, err := handler.Handle(t.Context(), "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQueryMetrics_Error(t *testing.T) {
	reg := prometheus.NewRegistry()
	errTest := errors.New("metrics query fail")
	inner := &mockQueryHandler[string, string]{
		fn: func(ctx context.Context, query string) (string, error) {
			return "", errTest
		},
	}

	mw := QueryMetrics[string, string]("test_query", reg)
	handler := mw(inner)

	_, err := handler.Handle(t.Context(), "q")
	if !errors.Is(err, errTest) {
		t.Fatalf("expected errTest, got %v", err)
	}
}

func TestCommandTracing(t *testing.T) {
	inner := &mockCommandHandler[string, string]{
		fn: func(ctx context.Context, cmd string) (string, string, error) {
			return cmd, "traced", nil
		},
	}

	mw := CommandTracing[string, string]("TestSpan")
	handler := mw(inner)

	cmd, resp, err := handler.Handle(t.Context(), "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "x" || resp != "traced" {
		t.Fatalf("got cmd=%q resp=%q", cmd, resp)
	}
}

func TestQueryTracing(t *testing.T) {
	inner := &mockQueryHandler[string, string]{
		fn: func(ctx context.Context, query string) (string, error) {
			return "traced", nil
		},
	}

	mw := QueryTracing[string, string]("QuerySpan")
	handler := mw(inner)

	resp, err := handler.Handle(t.Context(), "q")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "traced" {
		t.Fatalf("got %q, want %q", resp, "traced")
	}
}

func TestChainCommandWithMiddleware(t *testing.T) {
	log := testx.NopLogger()
	reg := prometheus.NewRegistry()
	inner := &mockCommandHandler[string, string]{
		fn: func(ctx context.Context, cmd string) (string, string, error) {
			return cmd, "chained", nil
		},
	}

	handler := cqrs.ChainCommand[string, string](
		inner,
		CommandLogging[string, string](log, "ChainTest"),
		CommandMetrics[string, string]("chain_cmd", reg),
		CommandTracing[string, string]("ChainSpan"),
	)

	_, resp, err := handler.Handle(t.Context(), "chain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "chained" {
		t.Fatalf("got %q, want %q", resp, "chained")
	}
}
