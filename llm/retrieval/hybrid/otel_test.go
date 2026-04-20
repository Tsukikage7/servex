package hybrid

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/Tsukikage7/servex/v2/llm/retrieval/rag"
)

// installSpanRecorder 安装 tracetest.SpanRecorder 返回 recorder,测试结束时恢复全局 TracerProvider.
func installSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return rec
}

// findSpan 按名称在 ended spans 里查找第一个匹配项.
func findSpan(spans []sdktrace.ReadOnlySpan, name string) sdktrace.ReadOnlySpan {
	for _, s := range spans {
		if s.Name() == name {
			return s
		}
	}
	return nil
}

// attrInt 从 span 读取 int64 属性值,找不到返回 (_, false).
func attrInt(s sdktrace.ReadOnlySpan, key string) (int64, bool) {
	for _, kv := range s.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsInt64(), true
		}
	}
	return 0, false
}

// attrFloat 从 span 读取 float64 属性值.
func attrFloat(s sdktrace.ReadOnlySpan, key string) (float64, bool) {
	for _, kv := range s.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsFloat64(), true
		}
	}
	return 0, false
}

// TestHybridRetrieverEmitsSpan 验证 Hybrid.Retrieve 产生 hybrid.Retrieve span + 其属性,
// 并且 BM25 词法路作为其子 span.
func TestHybridRetrieverEmitsSpan(t *testing.T) {
	rec := installSpanRecorder(t)

	// 向量路:mockRetriever(不发 span) / 词法路:BM25Retriever(发 bm25.Retrieve).
	vec := &mockRetriever{docs: makeDocs("vecA", "vecB")}
	lex := NewBM25Retriever([]rag.Document{
		{ID: "d1", Content: "退款 政策"},
		{ID: "d2", Content: "发货 物流"},
	})

	h := New(vec, lex, WithRRFK(60), WithWeights(1.0, 0.5))
	_, err := h.Retrieve(context.Background(), "退款", 3)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	spans := rec.Ended()
	hybridSpan := findSpan(spans, "hybrid.Retrieve")
	if hybridSpan == nil {
		t.Fatal("缺少 hybrid.Retrieve span")
	}

	// 校验属性存在.
	if v, ok := attrInt(hybridSpan, "hybrid.query_len"); !ok || v != int64(len("退款")) {
		t.Errorf("hybrid.query_len 不符: got %d ok=%v", v, ok)
	}
	if v, ok := attrInt(hybridSpan, "hybrid.top_k"); !ok || v != 3 {
		t.Errorf("hybrid.top_k 不符: got %d ok=%v", v, ok)
	}
	if v, ok := attrInt(hybridSpan, "hybrid.rrf_k"); !ok || v != 60 {
		t.Errorf("hybrid.rrf_k 不符: got %d ok=%v", v, ok)
	}
	if v, ok := attrFloat(hybridSpan, "hybrid.vec_weight"); !ok || v != 1.0 {
		t.Errorf("hybrid.vec_weight 不符: got %f ok=%v", v, ok)
	}
	if v, ok := attrFloat(hybridSpan, "hybrid.lex_weight"); !ok || v != 0.5 {
		t.Errorf("hybrid.lex_weight 不符: got %f ok=%v", v, ok)
	}
	if _, ok := attrInt(hybridSpan, "hybrid.hits"); !ok {
		t.Error("缺少 hybrid.hits 属性")
	}
	if hybridSpan.Status().Code != codes.Unset && hybridSpan.Status().Code != codes.Ok {
		t.Errorf("期望成功 Status,得到 %s", hybridSpan.Status().Code)
	}

	// 校验 BM25 span 作为 hybrid 子 span.
	bmSpan := findSpan(spans, "bm25.Retrieve")
	if bmSpan == nil {
		t.Fatal("期望 bm25.Retrieve 作为子 span")
	}
	if bmSpan.Parent().SpanID() != hybridSpan.SpanContext().SpanID() {
		t.Errorf("bm25 span 不是 hybrid span 的子 span: parent=%s, hybrid=%s",
			bmSpan.Parent().SpanID(), hybridSpan.SpanContext().SpanID())
	}
}

// TestBM25RetrieverEmitsSpan 直接调用 BM25.Retrieve 时也产生 span.
func TestBM25RetrieverEmitsSpan(t *testing.T) {
	rec := installSpanRecorder(t)

	r := NewBM25Retriever([]rag.Document{
		{ID: "d1", Content: "hello world"},
		{ID: "d2", Content: "foo bar"},
	})
	_, err := r.Retrieve(context.Background(), "hello", 5)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	s := findSpan(rec.Ended(), "bm25.Retrieve")
	if s == nil {
		t.Fatal("缺少 bm25.Retrieve span")
	}
	if v, ok := attrInt(s, "bm25.query_len"); !ok || v != int64(len("hello")) {
		t.Errorf("bm25.query_len 不符: got %d ok=%v", v, ok)
	}
	if v, ok := attrInt(s, "bm25.top_k"); !ok || v != 5 {
		t.Errorf("bm25.top_k 不符: got %d ok=%v", v, ok)
	}
	if v, ok := attrInt(s, "bm25.docs_count"); !ok || v != 2 {
		t.Errorf("bm25.docs_count 不符: got %d ok=%v", v, ok)
	}
	if _, ok := attrInt(s, "bm25.hits"); !ok {
		t.Error("缺少 bm25.hits 属性")
	}
	// 根 span(调用方无父 span).
	if s.Parent().IsValid() && s.Parent().SpanID() != (trace.SpanID{}) {
		// 作为顶层调用,父 SpanContext 应当无效.
		t.Errorf("期望 bm25.Retrieve 为根 span,实际父 SpanID=%s", s.Parent().SpanID())
	}
}

// TestHybridRetrieverSpanErrorOnBothFail 两路都失败时 hybrid span Status=Error.
func TestHybridRetrieverSpanErrorOnBothFail(t *testing.T) {
	rec := installSpanRecorder(t)

	errVec := errors.New("vec fail")
	errLex := errors.New("lex fail")
	vec := &mockRetriever{err: errVec}
	lex := &mockRetriever{err: errLex}

	h := New(vec, lex)
	_, err := h.Retrieve(context.Background(), "q", 5)
	if err == nil {
		t.Fatal("期望错误")
	}

	s := findSpan(rec.Ended(), "hybrid.Retrieve")
	if s == nil {
		t.Fatal("缺少 hybrid.Retrieve span")
	}
	if s.Status().Code != codes.Error {
		t.Errorf("期望 Status=Error,得到 %s", s.Status().Code)
	}
}

// TestHybridRetrieverSpanOkWhenOnePathFails 一路失败时整体仍视为成功,span Status 不是 Error.
func TestHybridRetrieverSpanOkWhenOnePathFails(t *testing.T) {
	rec := installSpanRecorder(t)

	vec := &mockRetriever{err: errors.New("vec fail")}
	lex := &mockRetriever{docs: makeDocs("lexA", "lexB")}

	h := New(vec, lex)
	_, err := h.Retrieve(context.Background(), "q", 5)
	if err != nil {
		t.Fatalf("期望 nil 错误,实际: %v", err)
	}

	s := findSpan(rec.Ended(), "hybrid.Retrieve")
	if s == nil {
		t.Fatal("缺少 hybrid.Retrieve span")
	}
	if s.Status().Code == codes.Error {
		t.Errorf("期望非 Error 状态,得到 %s", s.Status().Code)
	}
	// hybrid.hits 应当被设置为 2.
	if v, ok := attrInt(s, "hybrid.hits"); !ok || v != 2 {
		t.Errorf("hybrid.hits 不符: got %d ok=%v", v, ok)
	}
}
