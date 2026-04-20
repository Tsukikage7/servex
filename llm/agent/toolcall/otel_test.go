package toolcall_test

import (
	"context"
	"encoding/json"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/agent/toolcall"
)

func installToolcallSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return rec
}

// 工具循环有工具调用时,期望出现 toolcall.Run 与 toolcall.Execute 两类 span.
func TestToolcall_Run_EmitsSpans(t *testing.T) {
	rec := installToolcallSpanRecorder(t)

	tc := llm.ToolCall{ID: "c1"}
	tc.Function.Name = "get_time"
	tc.Function.Arguments = `{}`

	model := &mockToolModel{
		rounds: []llm.ChatResponse{
			{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{tc}}, FinishReason: "tool_calls"},
			{Message: llm.AssistantMessage("done"), FinishReason: "stop"},
		},
	}
	reg := toolcall.NewRegistry()
	reg.Register(
		llm.Tool{Function: llm.FunctionDef{Name: "get_time"}},
		func(context.Context, string) (string, error) {
			result, _ := json.Marshal(map[string]string{"t": "noon"})
			return string(result), nil
		},
	)

	exec := toolcall.NewExecutor(model, reg)
	if _, err := exec.Run(context.Background(), []llm.Message{llm.UserMessage("hi")}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	names := map[string]int{}
	for _, s := range rec.Ended() {
		names[s.Name()]++
	}
	if names["toolcall.Run"] == 0 {
		t.Error("缺少 toolcall.Run span")
	}
	if names["toolcall.Execute"] == 0 {
		t.Error("缺少 toolcall.Execute span")
	}

	// 检查 toolcall.Execute 的关键属性.
	for _, s := range rec.Ended() {
		if s.Name() != "toolcall.Execute" {
			continue
		}
		var hasName, hasCallID, hasDuration bool
		for _, kv := range s.Attributes() {
			switch string(kv.Key) {
			case "tool.name":
				if kv.Value.AsString() == "get_time" {
					hasName = true
				}
			case "tool.call_id":
				if kv.Value.AsString() == "c1" {
					hasCallID = true
				}
			case "tool.duration_ms":
				hasDuration = true
			}
		}
		if !hasName {
			t.Error("缺少 tool.name=get_time")
		}
		if !hasCallID {
			t.Error("缺少 tool.call_id=c1")
		}
		if !hasDuration {
			t.Error("缺少 tool.duration_ms")
		}
	}
}
