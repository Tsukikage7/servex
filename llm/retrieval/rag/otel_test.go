package rag_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/retrieval/rag"
	"github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore"
)

// --- 测试辅助 ---

// fakeEmbed 返回指定向量的嵌入模型.
type fakeEmbed struct {
	vec []float32
	err error
}

func (f *fakeEmbed) EmbedTexts(_ context.Context, texts []string, _ ...llm.CallOption) (*llm.EmbedResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	embs := make([][]float32, len(texts))
	for i := range embs {
		embs[i] = f.vec
	}
	return &llm.EmbedResponse{Embeddings: embs}, nil
}

// fakeVectorStore 返回预设文档.
type fakeVectorStore struct {
	results []vectorstore.SearchResult
	err     error
}

func (f *fakeVectorStore) AddDocuments(context.Context, []vectorstore.Document) error {
	return nil
}

func (f *fakeVectorStore) SimilaritySearch(_ context.Context, _ []float32, _ int, _ ...vectorstore.SearchOption) ([]vectorstore.SearchResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.results, nil
}

func (f *fakeVectorStore) Delete(context.Context, []string) error { return nil }
func (f *fakeVectorStore) Close() error                            { return nil }

// fakeChat 用于 Query/QueryStream 路径.
type fakeChat struct{}

func (f *fakeChat) Generate(context.Context, []llm.Message, ...llm.CallOption) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Message: llm.AssistantMessage("ok")}, nil
}

func (f *fakeChat) Stream(context.Context, []llm.Message, ...llm.CallOption) (llm.StreamReader, error) {
	return &stubStream{}, nil
}

// stubStream 最小 StreamReader:Recv 立即 EOF.
type stubStream struct{ done bool }

func (s *stubStream) Recv() (llm.StreamChunk, error) {
	if s.done {
		return llm.StreamChunk{}, io.EOF
	}
	s.done = true
	return llm.StreamChunk{}, io.EOF
}
func (s *stubStream) Response() *llm.ChatResponse { return nil }
func (s *stubStream) Close() error                { return nil }

// installSpanRecorder 安装 tracetest.SpanRecorder 返回 recorder 与清理函数.
func installSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return rec
}

// --- 测试 ---

func TestRagRetrieveEmitsSpan(t *testing.T) {
	rec := installSpanRecorder(t)

	p, err := rag.New(&rag.Config{
		ChatModel:      &fakeChat{},
		EmbeddingModel: &fakeEmbed{vec: []float32{0.1, 0.2}},
		VectorStore: &fakeVectorStore{
			results: []vectorstore.SearchResult{
				{Document: vectorstore.Document{ID: "d1", Content: "hello"}, Score: 0.9},
			},
		},
		TopK: 3,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := p.Retrieve(context.Background(), "hi"); err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	spans := rec.Ended()
	if len(spans) == 0 {
		t.Fatal("期望至少一个 span")
	}
	found := false
	for _, s := range spans {
		if s.Name() == "rag.Retrieve" {
			found = true
			// 检查属性
			attrs := s.Attributes()
			var hasTopK, hasQLen bool
			for _, kv := range attrs {
				if string(kv.Key) == "rag.top_k" && kv.Value.AsInt64() == 3 {
					hasTopK = true
				}
				if string(kv.Key) == "rag.question_len" && kv.Value.AsInt64() == 2 {
					hasQLen = true
				}
			}
			if !hasTopK {
				t.Error("缺少 rag.top_k=3")
			}
			if !hasQLen {
				t.Error("缺少 rag.question_len=2")
			}
		}
	}
	if !found {
		t.Error("未找到 rag.Retrieve span")
	}
}

func TestRagRetrieveSpanErrorOnEmbedFailure(t *testing.T) {
	rec := installSpanRecorder(t)

	targetErr := errors.New("embed failed")
	p, _ := rag.New(&rag.Config{
		ChatModel:      &fakeChat{},
		EmbeddingModel: &fakeEmbed{err: targetErr},
		VectorStore:    &fakeVectorStore{},
	})
	if _, err := p.Retrieve(context.Background(), "q"); err == nil {
		t.Fatal("期望错误")
	}
	spans := rec.Ended()
	for _, s := range spans {
		if s.Name() == "rag.Retrieve" {
			if s.Status().Code.String() != "Error" {
				t.Errorf("期望 span 状态为 Error,得到 %s", s.Status().Code)
			}
			return
		}
	}
	t.Error("未找到 rag.Retrieve span")
}

func TestRagQueryEmitsSpan(t *testing.T) {
	rec := installSpanRecorder(t)

	p, _ := rag.New(&rag.Config{
		ChatModel:      &fakeChat{},
		EmbeddingModel: &fakeEmbed{vec: []float32{0.1}},
		VectorStore: &fakeVectorStore{
			results: []vectorstore.SearchResult{
				{Document: vectorstore.Document{ID: "d1", Content: "hello"}, Score: 0.9},
			},
		},
	})
	if _, err := p.Query(context.Background(), "hi"); err != nil {
		t.Fatalf("Query: %v", err)
	}
	spans := rec.Ended()
	names := make(map[string]bool)
	for _, s := range spans {
		names[s.Name()] = true
	}
	if !names["rag.Query"] {
		t.Error("缺少 rag.Query span")
	}
	if !names["rag.Retrieve"] {
		t.Error("期望 Query 路径内嵌 rag.Retrieve")
	}
}

func TestRagQueryStreamEmitsSpan(t *testing.T) {
	rec := installSpanRecorder(t)

	p, _ := rag.New(&rag.Config{
		ChatModel:      &fakeChat{},
		EmbeddingModel: &fakeEmbed{vec: []float32{0.1}},
		VectorStore: &fakeVectorStore{
			results: []vectorstore.SearchResult{
				{Document: vectorstore.Document{ID: "d1", Content: "hi"}, Score: 0.8},
			},
		},
	})
	reader, err := p.QueryStream(context.Background(), "hi")
	if err != nil {
		t.Fatalf("QueryStream: %v", err)
	}
	_ = reader.Close()

	names := make(map[string]bool)
	for _, s := range rec.Ended() {
		names[s.Name()] = true
	}
	if !names["rag.QueryStream"] {
		t.Error("缺少 rag.QueryStream span")
	}
	if !names["rag.Retrieve"] {
		t.Error("期望 QueryStream 路径内嵌 rag.Retrieve")
	}
}
