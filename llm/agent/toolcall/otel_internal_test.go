package toolcall

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/Tsukikage7/servex/v2/llm"
)

func installRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return rec
}

// 工具 fn panic:
//  1. panic 必须被 rethrow(不能被吞).
//  2. span 必须被关闭,状态为 Error,且包含 tool.duration_ms.
func TestWithToolCallSpan_PanicIsRethrownAndSpanClosed(t *testing.T) {
	rec := installRecorder(t)

	tc := llm.ToolCall{ID: "panic-id"}
	tc.Function.Name = "panicker"

	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_, _ = withToolCallSpan(context.Background(), tc, func(context.Context) (string, error) {
			panic("boom")
		})
	}()

	if !panicked {
		t.Fatal("panic 应被 rethrow,调用侧未观察到")
	}

	// span 检查.
	spans := rec.Ended()
	if len(spans) == 0 {
		t.Fatal("期望至少一个 ended span(panic 路径也要关)")
	}
	var found bool
	for _, s := range spans {
		if s.Name() != "toolcall.Execute" {
			continue
		}
		found = true
		if s.Status().Code.String() != "Error" {
			t.Errorf("panic 路径期望 Error status,得到 %s", s.Status().Code)
		}
		var hasDuration bool
		for _, kv := range s.Attributes() {
			if string(kv.Key) == "tool.duration_ms" {
				hasDuration = true
			}
		}
		if !hasDuration {
			t.Error("panic 路径仍应记录 tool.duration_ms")
		}
	}
	if !found {
		t.Fatal("未找到 toolcall.Execute span")
	}
}

// 正常错误(非 panic)路径:span 状态 Error,duration 被记录.
func TestWithToolCallSpan_ErrorPath(t *testing.T) {
	rec := installRecorder(t)
	tc := llm.ToolCall{ID: "err-id"}
	tc.Function.Name = "failer"

	want := errors.New("tool failed")
	_, err := withToolCallSpan(context.Background(), tc, func(context.Context) (string, error) {
		return "", want
	})
	if !errors.Is(err, want) {
		t.Fatalf("期望错误透传,得到 %v", err)
	}
	for _, s := range rec.Ended() {
		if s.Name() == "toolcall.Execute" {
			if s.Status().Code.String() != "Error" {
				t.Errorf("期望 Error status,得到 %s", s.Status().Code)
			}
			return
		}
	}
	t.Fatal("未找到 toolcall.Execute span")
}
