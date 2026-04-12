package rag

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/Tsukikage7/servex/llm"
	"github.com/Tsukikage7/servex/llm/retrieval/splitter"
	"github.com/Tsukikage7/servex/llm/retrieval/vectorstore"
)

// ──────────────────────────────────────────
// Mock 实现
// ──────────────────────────────────────────

// mockChatModel 模拟聊天模型.
type mockChatModel struct {
	// generateFn 可选的自定义 Generate 行为.
	generateFn func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error)
}

func (m *mockChatModel) Generate(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	if m.generateFn != nil {
		return m.generateFn(ctx, messages, opts...)
	}
	return &llm.ChatResponse{
		Message: llm.AssistantMessage("mock answer"),
		Usage:   llm.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}, nil
}

func (m *mockChatModel) Stream(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
	return &mockStreamReader{}, nil
}

// mockStreamReader 模拟流式读取器.
type mockStreamReader struct {
	done bool
}

func (r *mockStreamReader) Recv() (llm.StreamChunk, error) {
	if r.done {
		return llm.StreamChunk{}, io.EOF
	}
	r.done = true
	return llm.StreamChunk{Delta: "mock stream chunk"}, nil
}

func (r *mockStreamReader) Response() *llm.ChatResponse {
	if !r.done {
		return nil
	}
	return &llm.ChatResponse{Message: llm.AssistantMessage("mock stream chunk")}
}

func (r *mockStreamReader) Close() error { return nil }

// mockEmbeddingModel 模拟嵌入模型.
type mockEmbeddingModel struct {
	// embedFn 可选的自定义 EmbedTexts 行为.
	embedFn func(ctx context.Context, texts []string, opts ...llm.CallOption) (*llm.EmbedResponse, error)
}

func (m *mockEmbeddingModel) EmbedTexts(ctx context.Context, texts []string, opts ...llm.CallOption) (*llm.EmbedResponse, error) {
	if m.embedFn != nil {
		return m.embedFn(ctx, texts, opts...)
	}
	// 为每个文本生成一个简单的固定向量.
	embeddings := make([][]float32, len(texts))
	for i := range embeddings {
		embeddings[i] = []float32{0.1, 0.2, 0.3}
	}
	return &llm.EmbedResponse{
		Embeddings: embeddings,
		ModelID:    "mock-embedding-model",
	}, nil
}

// mockVectorStore 模拟向量存储.
type mockVectorStore struct {
	// docs 存储已添加的文档.
	docs []vectorstore.Document
	// searchFn 可选的自定义 SimilaritySearch 行为.
	searchFn func(ctx context.Context, query []float32, k int, opts ...vectorstore.SearchOption) ([]vectorstore.SearchResult, error)
}

func (s *mockVectorStore) AddDocuments(ctx context.Context, docs []vectorstore.Document) error {
	s.docs = append(s.docs, docs...)
	return nil
}

func (s *mockVectorStore) SimilaritySearch(ctx context.Context, query []float32, k int, opts ...vectorstore.SearchOption) ([]vectorstore.SearchResult, error) {
	if s.searchFn != nil {
		return s.searchFn(ctx, query, k, opts...)
	}
	// 默认返回存储中前 k 条文档.
	n := k
	if n > len(s.docs) {
		n = len(s.docs)
	}
	results := make([]vectorstore.SearchResult, n)
	for i := 0; i < n; i++ {
		results[i] = vectorstore.SearchResult{
			Document: s.docs[i],
			Score:    0.9,
		}
	}
	return results, nil
}

func (s *mockVectorStore) Delete(ctx context.Context, ids []string) error { return nil }

// ──────────────────────────────────────────
// TestNew_Validation
// ──────────────────────────────────────────

// TestNew_Validation 验证 New 对必填字段的校验逻辑.
func TestNew_Validation(t *testing.T) {
	t.Run("nil chat model", func(t *testing.T) {
		_, err := New(&Config{
			EmbeddingModel: &mockEmbeddingModel{},
			VectorStore:    &mockVectorStore{},
		})
		if err != ErrNilChatModel {
			t.Fatalf("期望 ErrNilChatModel，实际得到: %v", err)
		}
	})

	t.Run("nil embedding model", func(t *testing.T) {
		_, err := New(&Config{
			ChatModel:   &mockChatModel{},
			VectorStore: &mockVectorStore{},
		})
		if err != ErrNilEmbeddingModel {
			t.Fatalf("期望 ErrNilEmbeddingModel，实际得到: %v", err)
		}
	})

	t.Run("nil vector store", func(t *testing.T) {
		_, err := New(&Config{
			ChatModel:      &mockChatModel{},
			EmbeddingModel: &mockEmbeddingModel{},
		})
		if err != ErrNilVectorStore {
			t.Fatalf("期望 ErrNilVectorStore，实际得到: %v", err)
		}
	})

	t.Run("valid config", func(t *testing.T) {
		p, err := New(&Config{
			ChatModel:      &mockChatModel{},
			EmbeddingModel: &mockEmbeddingModel{},
			VectorStore:    &mockVectorStore{},
		})
		if err != nil {
			t.Fatalf("期望成功，实际得到错误: %v", err)
		}
		if p == nil {
			t.Fatal("期望返回非 nil Pipeline")
		}
		// 默认 TopK 应为 5.
		if p.cfg.TopK != 5 {
			t.Fatalf("期望默认 TopK=5，实际得到: %d", p.cfg.TopK)
		}
		// 应自动创建默认提示词模板.
		if p.cfg.PromptTemplate == nil {
			t.Fatal("期望默认提示词模板非 nil")
		}
	})
}

// ──────────────────────────────────────────
// TestIngest
// ──────────────────────────────────────────

// TestIngest 验证 Ingest 正确将文档嵌入并存入向量库.
func TestIngest(t *testing.T) {
	vs := &mockVectorStore{}
	p, err := New(&Config{
		ChatModel:      &mockChatModel{},
		EmbeddingModel: &mockEmbeddingModel{},
		VectorStore:    vs,
	})
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}

	docs := []Document{
		{ID: "doc1", Content: "第一篇文档内容"},
		{ID: "doc2", Content: "第二篇文档内容"},
	}

	if err := p.Ingest(t.Context(), docs); err != nil {
		t.Fatalf("Ingest 失败: %v", err)
	}

	// 验证向量库中收到的文档数量.
	if len(vs.docs) != 2 {
		t.Fatalf("期望向量库中有 2 条文档，实际有 %d 条", len(vs.docs))
	}

	// 验证文档 ID 和内容.
	if vs.docs[0].ID != "doc1" {
		t.Errorf("期望第一条文档 ID=doc1，实际=%s", vs.docs[0].ID)
	}
	if vs.docs[1].ID != "doc2" {
		t.Errorf("期望第二条文档 ID=doc2，实际=%s", vs.docs[1].ID)
	}

	// 验证向量已写入.
	if len(vs.docs[0].Vector) == 0 {
		t.Error("期望文档包含嵌入向量，实际为空")
	}
}

// ──────────────────────────────────────────
// TestIngest_WithSplitter
// ──────────────────────────────────────────

// TestIngest_WithSplitter 验证启用分块器时文档被正确拆分.
func TestIngest_WithSplitter(t *testing.T) {
	vs := &mockVectorStore{}
	// 使用字符分块器，每块 5 个字符，无重叠.
	sp := splitter.NewCharacterSplitter(
		splitter.WithChunkSize(5),
		splitter.WithChunkOverlap(0),
	)

	p, err := New(&Config{
		ChatModel:      &mockChatModel{},
		EmbeddingModel: &mockEmbeddingModel{},
		VectorStore:    vs,
		Splitter:       sp,
	})
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}

	// 20 个字符的文档，按 5 字符分块 → 4 块.
	docs := []Document{
		{ID: "doc1", Content: "一二三四五六七八九十甲乙丙丁戊己庚辛壬癸"},
	}

	if err := p.Ingest(t.Context(), docs); err != nil {
		t.Fatalf("Ingest 失败: %v", err)
	}

	// 20 个 CJK 字符 / 5 字符每块 = 4 块.
	if len(vs.docs) != 4 {
		t.Fatalf("期望 4 个分块，实际得到 %d 个", len(vs.docs))
	}

	// 验证分块 ID 格式为 docID_chunkIndex.
	for i, doc := range vs.docs {
		expectedID := fmt.Sprintf("doc1_%d", i)
		if doc.ID != expectedID {
			t.Errorf("期望分块 ID=%s，实际=%s", expectedID, doc.ID)
		}
	}

	// 验证元数据中包含来源文档 ID.
	if vs.docs[0].Metadata["source_doc_id"] != "doc1" {
		t.Errorf("期望 source_doc_id=doc1，实际=%v", vs.docs[0].Metadata["source_doc_id"])
	}
}

// ──────────────────────────────────────────
// TestRetrieve
// ──────────────────────────────────────────

// TestRetrieve 验证 Retrieve 以正确参数调用向量搜索.
func TestRetrieve(t *testing.T) {
	var capturedK int
	var capturedQuery []float32
	var capturedOpts []vectorstore.SearchOption

	vs := &mockVectorStore{
		docs: []vectorstore.Document{
			{ID: "doc1", Content: "相关内容", Vector: []float32{0.1, 0.2, 0.3}},
			{ID: "doc2", Content: "另一段内容", Vector: []float32{0.4, 0.5, 0.6}},
		},
		searchFn: func(ctx context.Context, query []float32, k int, opts ...vectorstore.SearchOption) ([]vectorstore.SearchResult, error) {
			capturedK = k
			capturedQuery = query
			capturedOpts = opts
			return []vectorstore.SearchResult{
				{Document: vectorstore.Document{ID: "doc1", Content: "相关内容"}, Score: 0.95},
			}, nil
		},
	}

	p, err := New(&Config{
		ChatModel:      &mockChatModel{},
		EmbeddingModel: &mockEmbeddingModel{},
		VectorStore:    vs,
		TopK:           3,
		ScoreThreshold: 0.8,
	})
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}

	results, err := p.Retrieve(t.Context(), "测试问题")
	if err != nil {
		t.Fatalf("Retrieve 失败: %v", err)
	}

	// 验证传入的 TopK.
	if capturedK != 3 {
		t.Errorf("期望 k=3，实际=%d", capturedK)
	}

	// 验证查询向量非空.
	if len(capturedQuery) == 0 {
		t.Error("期望查询向量非空")
	}

	// 验证 ScoreThreshold 被传递（应存在搜索选项）.
	if len(capturedOpts) == 0 {
		t.Error("期望传递 ScoreThreshold 搜索选项")
	}

	// 验证返回结果.
	if len(results) != 1 {
		t.Fatalf("期望 1 条检索结果，实际得到 %d 条", len(results))
	}
	if results[0].ID != "doc1" {
		t.Errorf("期望检索到 doc1，实际=%s", results[0].ID)
	}
	if results[0].Score != 0.95 {
		t.Errorf("期望 Score=0.95，实际=%f", results[0].Score)
	}
}

// TestQuery 验证完整 RAG 管线（检索 + 生成）.
func TestQuery(t *testing.T) {
	var capturedMessages []llm.Message

	chat := &mockChatModel{
		generateFn: func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
			capturedMessages = messages
			return &llm.ChatResponse{
				Message: llm.AssistantMessage("这是生成的回答"),
				Usage:   llm.Usage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30},
			}, nil
		},
	}

	vs := &mockVectorStore{
		docs: []vectorstore.Document{
			{ID: "doc1", Content: "参考内容一", Vector: []float32{0.1, 0.2, 0.3}},
		},
	}

	p, err := New(&Config{
		ChatModel:      chat,
		EmbeddingModel: &mockEmbeddingModel{},
		VectorStore:    vs,
	})
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}

	result, err := p.Query(t.Context(), "用户的问题")
	if err != nil {
		t.Fatalf("Query 失败: %v", err)
	}

	// 验证回答内容.
	if result.Answer != "这是生成的回答" {
		t.Errorf("期望回答='这是生成的回答'，实际='%s'", result.Answer)
	}

	// 验证 Sources 非空.
	if len(result.Sources) == 0 {
		t.Error("期望 Sources 非空")
	}

	// 验证 Usage 被正确传递.
	if result.Usage.TotalTokens != 30 {
		t.Errorf("期望 TotalTokens=30，实际=%d", result.Usage.TotalTokens)
	}

	// 验证向聊天模型传递了系统消息和用户消息.
	if len(capturedMessages) < 2 {
		t.Fatalf("期望至少传递 2 条消息（系统+用户），实际传递 %d 条", len(capturedMessages))
	}
	if capturedMessages[0].Role != llm.RoleSystem {
		t.Errorf("期望第一条消息为系统消息，实际 Role=%s", capturedMessages[0].Role)
	}
	if capturedMessages[1].Role != llm.RoleUser {
		t.Errorf("期望第二条消息为用户消息，实际 Role=%s", capturedMessages[1].Role)
	}
}
