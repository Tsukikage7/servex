package hybrid

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// tracerName hybrid 子包统一的 tracer 名称.
const tracerName = "servex.llm.retrieval.hybrid"

// hybridTracer 返回包级 tracer.
// 每次获取都走 otel.Tracer 以兼容运行时切换 TracerProvider.
func hybridTracer() trace.Tracer {
	return otel.Tracer(tracerName)
}

// startHybridRetrieveSpan 开启混合检索 span,返回 ctx 与 span.
// 属性:hybrid.query_len / hybrid.top_k / hybrid.rrf_k / hybrid.vec_weight / hybrid.lex_weight.
func startHybridRetrieveSpan(ctx context.Context, query string, topK int, rrfK int, vw, lw float32) (context.Context, trace.Span) {
	return hybridTracer().Start(ctx, "hybrid.Retrieve",
		trace.WithAttributes(
			attribute.Int("hybrid.query_len", len(query)),
			attribute.Int("hybrid.top_k", topK),
			attribute.Int("hybrid.rrf_k", rrfK),
			attribute.Float64("hybrid.vec_weight", float64(vw)),
			attribute.Float64("hybrid.lex_weight", float64(lw)),
		),
	)
}

// startBM25RetrieveSpan 开启 BM25 span,返回 ctx 与 span.
// 属性:bm25.query_len / bm25.top_k / bm25.docs_count.
func startBM25RetrieveSpan(ctx context.Context, query string, topK int, docsCount int) (context.Context, trace.Span) {
	return hybridTracer().Start(ctx, "bm25.Retrieve",
		trace.WithAttributes(
			attribute.Int("bm25.query_len", len(query)),
			attribute.Int("bm25.top_k", topK),
			attribute.Int("bm25.docs_count", docsCount),
		),
	)
}

// recordHybridResult 成功结束:记录 hybrid.hits 属性.
func recordHybridResult(span trace.Span, hits int) {
	span.SetAttributes(attribute.Int("hybrid.hits", hits))
}

// recordBM25Result 成功结束:记录 bm25.hits 属性.
func recordBM25Result(span trace.Span, hits int) {
	span.SetAttributes(attribute.Int("bm25.hits", hits))
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
