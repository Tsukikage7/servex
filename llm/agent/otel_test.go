package agent

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/Tsukikage7/servex/v2/llm"
)

func installAgentSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return rec
}

// 检查 Run 成功路径会发射 agent.Run span,并携带 iterations/tool_calls_count 属性.
func TestAgent_Run_EmitsSpan(t *testing.T) {
	rec := installAgentSpanRecorder(t)

	model := &mockModel{
		responses: []*llm.ChatResponse{{
			Message:      llm.AssistantMessage("final"),
			FinishReason: "stop",
		}},
	}
	a, err := New(&Config{Name: "t-agent", Model: model})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := a.Run(context.Background(), "hello"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var found bool
	for _, s := range rec.Ended() {
		if s.Name() != "agent.Run" {
			continue
		}
		found = true
		// 验证属性存在(具体值依赖策略执行结果,这里只做 best-effort 检查).
		var hasName, hasIter bool
		for _, kv := range s.Attributes() {
			switch string(kv.Key) {
			case "agent.name":
				if kv.Value.AsString() == "t-agent" {
					hasName = true
				}
			case "agent.iterations":
				hasIter = true
			}
		}
		if !hasName {
			t.Error("缺少 agent.name=t-agent")
		}
		if !hasIter {
			t.Error("缺少 agent.iterations")
		}
	}
	if !found {
		t.Fatal("未找到 agent.Run span")
	}
}

// 检查 Run 错误路径(model 失败)会将 span 置 Error.
func TestAgent_Run_SpanErrorOnFailure(t *testing.T) {
	rec := installAgentSpanRecorder(t)

	// 没有预置响应,model 会返回 "no more responses".
	model := &mockModel{}
	a, _ := New(&Config{Name: "err-agent", Model: model})
	_, _ = a.Run(context.Background(), "hello")

	var found bool
	for _, s := range rec.Ended() {
		if s.Name() == "agent.Run" {
			found = true
			if s.Status().Code.String() != "Error" {
				t.Errorf("期望 Error 状态,得到 %s", s.Status().Code)
			}
		}
	}
	if !found {
		t.Fatal("未找到 agent.Run span")
	}
}
