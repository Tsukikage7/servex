// OTel 集成 — tracer name、属性约定等见 tracerName 常量与 startRunSpan/startRunStreamSpan.
//
// 注:暂未记录 agent.model 属性 — Result 结构体当前不暴露模型 ID.
// 若后续 Result 增加 ModelID 字段,可在 recordAgentResult 中追加
// attribute.String("agent.model", result.ModelID).
package agent

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// tracerName Agent 子包统一的 tracer 名称.
const tracerName = "servex.llm.agent"

// agentTracer 返回包级 tracer.
func agentTracer() trace.Tracer {
	return otel.Tracer(tracerName)
}

// startRunSpan 为 Agent.Run 开启 span.
// 初始只记录 agent.name + agent.input_len;结束前由调用方补充 iterations / tool_calls_count / model.
func startRunSpan(ctx context.Context, name, input string) (context.Context, trace.Span) {
	return agentTracer().Start(ctx, "agent.Run",
		trace.WithAttributes(
			attribute.String("agent.name", name),
			attribute.Int("agent.input_len", len(input)),
		),
	)
}

// startRunStreamSpan 为 Agent.RunStream 开启 span.
func startRunStreamSpan(ctx context.Context, name, input string) (context.Context, trace.Span) {
	return agentTracer().Start(ctx, "agent.RunStream",
		trace.WithAttributes(
			attribute.String("agent.name", name),
			attribute.Int("agent.input_len", len(input)),
		),
	)
}

// recordAgentResult 写入 agent 执行完成的指标.
func recordAgentResult(span trace.Span, result *Result) {
	if result == nil {
		return
	}
	span.SetAttributes(
		attribute.Int("agent.iterations", result.Iterations),
		attribute.Int("agent.tool_calls_count", len(result.ToolCalls)),
	)
}

// recordAgentError 将 err 写入 span 并置 Status=Error.
func recordAgentError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
