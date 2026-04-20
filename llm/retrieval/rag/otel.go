package rag

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// tracerName RAG 子包统一的 tracer 名称.
const tracerName = "servex.llm.rag"

// tracer 返回包级 tracer.
// 每次获取都走 otel.Tracer 以兼容运行时切换 TracerProvider.
func ragTracer() trace.Tracer {
	return otel.Tracer(tracerName)
}

// startRetrieveSpan 开启 Retrieve 的 span,返回 ctx 与 span.
// 属性:rag.top_k / rag.question_len.
func startRetrieveSpan(ctx context.Context, question string, topK int) (context.Context, trace.Span) {
	return ragTracer().Start(ctx, "rag.Retrieve",
		trace.WithAttributes(
			attribute.Int("rag.top_k", topK),
			attribute.Int("rag.question_len", len(question)),
		),
	)
}

// startQuerySpan 开启 Query 的 span.
func startQuerySpan(ctx context.Context, question string, topK int) (context.Context, trace.Span) {
	return ragTracer().Start(ctx, "rag.Query",
		trace.WithAttributes(
			attribute.Int("rag.top_k", topK),
			attribute.Int("rag.question_len", len(question)),
		),
	)
}

// startQueryStreamSpan 开启 QueryStream 的 span.
// span 仅覆盖"检索 + 渲染 + 启动流式调用"阶段;返回 reader 后不再追踪流内 chunk,
// 避免需要包装 StreamReader 才能关闭 span.
func startQueryStreamSpan(ctx context.Context, question string, topK int) (context.Context, trace.Span) {
	return ragTracer().Start(ctx, "rag.QueryStream",
		trace.WithAttributes(
			attribute.Int("rag.top_k", topK),
			attribute.Int("rag.question_len", len(question)),
		),
	)
}

// recordSpanError 统一把 err 写入 span,并置 Status=Error.
// err 为 nil 时不做任何操作.
func recordSpanError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// ragHitsAttr 为 Retrieve 成功路径记录命中文档数.
func ragHitsAttr(n int) attribute.KeyValue {
	return attribute.Int("rag.hits", n)
}

// ragSourcesAttr 为 Query 成功路径记录用于生成的参考文档数.
func ragSourcesAttr(n int) attribute.KeyValue {
	return attribute.Int("rag.sources", n)
}
