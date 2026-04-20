package toolcall

import (
	"context"
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// tracerName 工具调用子包统一的 tracer 名称.
const tracerName = "servex.llm.toolcall"

// toolcallTracer 返回包级 tracer.
func toolcallTracer() trace.Tracer {
	return otel.Tracer(tracerName)
}

// startExecuteSpan 为 Executor.Run 开启 span,用于整个工具调用循环.
// 属性:toolcall.max_rounds.
func startExecuteSpan(ctx context.Context, maxRounds int) (context.Context, trace.Span) {
	return toolcallTracer().Start(ctx, "toolcall.Run",
		trace.WithAttributes(
			attribute.Int("toolcall.max_rounds", maxRounds),
		),
	)
}

// withToolCallSpan 为单次工具执行包裹 span.
// 属性:tool.name / tool.call_id / tool.duration_ms;错误时 RecordError 并置 Status=Error.
//
// panic 处理:内层工具 fn 若 panic,本函数会:
//  1. 在 span 上记录 "tool panic: ..." 错误和 Error 状态.
//  2. 补齐 tool.duration_ms 与 span.End().
//  3. 原样 rethrow panic,不吞掉(上层调用者/go runtime 决定处理方式).
func withToolCallSpan(ctx context.Context, call llm.ToolCall, fn func(context.Context) (string, error)) (out string, err error) {
	ctx, span := toolcallTracer().Start(ctx, "toolcall.Execute",
		trace.WithAttributes(
			attribute.String("tool.name", call.Function.Name),
			attribute.String("tool.call_id", call.ID),
		),
	)
	start := time.Now()

	defer func() {
		if r := recover(); r != nil {
			// panic 路径:记录错误,补齐属性,结束 span,再 rethrow.
			panicErr := fmt.Errorf("tool panic: %v", r)
			span.RecordError(panicErr)
			span.SetStatus(codes.Error, "panic")
			span.SetAttributes(attribute.Int64("tool.duration_ms", time.Since(start).Milliseconds()))
			span.End()
			panic(r)
		}
		// 正常路径:补齐属性并结束 span.
		span.SetAttributes(attribute.Int64("tool.duration_ms", time.Since(start).Milliseconds()))
		span.End()
	}()

	out, err = fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return out, err
}

// recordToolcallResult 写入工具循环完成的指标.
func recordToolcallResult(span trace.Span, rounds, toolCallCount int) {
	span.SetAttributes(
		attribute.Int("toolcall.rounds", rounds),
		attribute.Int("toolcall.tool_calls", toolCallCount),
	)
}

// recordToolcallError 统一错误写入.
func recordToolcallError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
