package rerank

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/retrieval/rag"
)

// ──────────────────────────────────────────
// Mock 实现
// ──────────────────────────────────────────

// mockChat 模拟聊天模型，通过 fn 自定义返回行为.
type mockChat struct {
	fn func(msgs []llm.Message) string
}

// Generate 实现 llm.ChatModel 接口，调用 fn 获取响应内容.
func (m *mockChat) Generate(_ context.Context, msgs []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
	content := ""
	if m.fn != nil {
		content = m.fn(msgs)
	}
	return &llm.ChatResponse{
		Message: llm.AssistantMessage(content),
	}, nil
}

// Stream 实现 llm.ChatModel 接口（测试中不使用）.
func (m *mockChat) Stream(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (llm.StreamReader, error) {
	return nil, fmt.Errorf("mockChat: 不支持 Stream")
}

// mockEmbedding 模拟嵌入模型，按调用顺序返回预设向量.
type mockEmbedding struct {
	// embeddings 预设的嵌入向量列表，按文本顺序返回.
	embeddings [][]float32
}

// EmbedTexts 实现 llm.EmbeddingModel 接口，返回预设的嵌入向量.
func (m *mockEmbedding) EmbedTexts(_ context.Context, texts []string, _ ...llm.CallOption) (*llm.EmbedResponse, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		if i < len(m.embeddings) {
			result[i] = m.embeddings[i]
		} else {
			result[i] = []float32{0, 0, 0}
		}
	}
	return &llm.EmbedResponse{Embeddings: result, ModelID: "mock-embed"}, nil
}

// ──────────────────────────────────────────
// 辅助函数
// ──────────────────────────────────────────

// makeDocs 根据 contents 创建 RetrievedDoc 列表，ID 为 doc0、doc1...
func makeDocs(contents ...string) []rag.RetrievedDoc {
	docs := make([]rag.RetrievedDoc, len(contents))
	for i, c := range contents {
		docs[i] = rag.RetrievedDoc{
			Document: rag.Document{ID: fmt.Sprintf("doc%d", i), Content: c},
			Score:    0,
		}
	}
	return docs
}

// ──────────────────────────────────────────
// TestLLMReranker
// ──────────────────────────────────────────

// TestLLMReranker 验证 LLMReranker 能够根据 LLM 返回的评分正确重排序文档.
func TestLLMReranker(t *testing.T) {
	// 预设三篇文档，LLM 返回 doc0=3, doc1=9, doc2=6
	// 期望排序：doc1, doc2, doc0.
	docs := makeDocs("文档A", "文档B", "文档C")

	chat := &mockChat{
		fn: func(msgs []llm.Message) string {
			// 返回全局索引的评分 JSON.
			return `[{"index":0,"score":3},{"index":1,"score":9},{"index":2,"score":6}]`
		},
	}

	reranker := NewLLMReranker(chat)
	result, err := reranker.Rerank(t.Context(), "测试查询", docs)
	if err != nil {
		t.Fatalf("Rerank 失败: %v", err)
	}

	// 验证返回数量.
	if len(result) != 3 {
		t.Fatalf("期望 3 条结果，实际 %d 条", len(result))
	}

	// 验证排序：doc1(score=9) > doc2(score=6) > doc0(score=3).
	expectedOrder := []string{"doc1", "doc2", "doc0"}
	for i, expected := range expectedOrder {
		if result[i].ID != expected {
			t.Errorf("位置 %d：期望 %s，实际 %s", i, expected, result[i].ID)
		}
	}

	// 验证分数被正确写入.
	if result[0].Score != 9 {
		t.Errorf("期望 result[0].Score=9，实际=%f", result[0].Score)
	}
}

// ──────────────────────────────────────────
// TestLLMReranker_WithTopN
// ──────────────────────────────────────────

// TestLLMReranker_WithTopN 验证 WithTopN 选项能够正确截取前 N 条结果.
func TestLLMReranker_WithTopN(t *testing.T) {
	docs := makeDocs("文档A", "文档B", "文档C", "文档D")

	chat := &mockChat{
		fn: func(msgs []llm.Message) string {
			return `[{"index":0,"score":5},{"index":1,"score":9},{"index":2,"score":7},{"index":3,"score":2}]`
		},
	}

	// 仅返回前 2 条.
	reranker := NewLLMReranker(chat, WithTopN(2))
	result, err := reranker.Rerank(t.Context(), "测试查询", docs)
	if err != nil {
		t.Fatalf("Rerank 失败: %v", err)
	}

	// 验证仅返回 2 条.
	if len(result) != 2 {
		t.Fatalf("期望 2 条结果，实际 %d 条", len(result))
	}

	// 验证前两名为 doc1(9) 和 doc2(7).
	if result[0].ID != "doc1" {
		t.Errorf("期望 result[0]=doc1，实际=%s", result[0].ID)
	}
	if result[1].ID != "doc2" {
		t.Errorf("期望 result[1]=doc2，实际=%s", result[1].ID)
	}
}

// ──────────────────────────────────────────
// TestEmbeddingReranker
// ──────────────────────────────────────────

// TestEmbeddingReranker 验证 EmbeddingReranker 能够根据余弦相似度正确重排序文档.
// 使用已知向量：
//   - query:  [1, 0, 0]
//   - doc0:   [0.9, 0.1, 0]  高相似度
//   - doc1:   [0.1, 0.9, 0]  低相似度
//   - doc2:   [0.7, 0.3, 0]  中等相似度
//
// 期望排序：doc0, doc2, doc1.
func TestEmbeddingReranker(t *testing.T) {
	docs := makeDocs("高相似文档", "低相似文档", "中等相似文档")

	// mockEmbedding 按 EmbedTexts 调用顺序依次返回：query, doc0, doc1, doc2.
	embed := &mockEmbedding{
		embeddings: [][]float32{
			{1, 0, 0},     // query
			{0.9, 0.1, 0}, // doc0
			{0.1, 0.9, 0}, // doc1
			{0.7, 0.3, 0}, // doc2
		},
	}

	reranker := NewEmbeddingReranker(embed)
	result, err := reranker.Rerank(t.Context(), "查询", docs)
	if err != nil {
		t.Fatalf("Rerank 失败: %v", err)
	}

	// 验证返回数量.
	if len(result) != 3 {
		t.Fatalf("期望 3 条结果，实际 %d 条", len(result))
	}

	// 验证排序：doc0(高) > doc2(中) > doc1(低).
	expectedOrder := []string{"doc0", "doc2", "doc1"}
	for i, expected := range expectedOrder {
		if result[i].ID != expected {
			t.Errorf("位置 %d：期望 %s，实际 %s（Score=%f）", i, expected, result[i].ID, result[i].Score)
		}
	}

	// 验证 Score 单调递减.
	for i := 1; i < len(result); i++ {
		if result[i].Score > result[i-1].Score {
			t.Errorf("Score 未单调递减：result[%d].Score=%f > result[%d].Score=%f",
				i, result[i].Score, i-1, result[i-1].Score)
		}
	}
}

// ──────────────────────────────────────────
// TestEmbeddingReranker_WithTopN
// ──────────────────────────────────────────

// TestEmbeddingReranker_WithTopN 验证 EmbeddingReranker 配合 WithTopN 选项能够正确截取前 N 条结果.
func TestEmbeddingReranker_WithTopN(t *testing.T) {
	docs := makeDocs("高相似文档", "低相似文档", "中等相似文档")

	embed := &mockEmbedding{
		embeddings: [][]float32{
			{1, 0, 0},     // query
			{0.9, 0.1, 0}, // doc0 高相似
			{0.1, 0.9, 0}, // doc1 低相似
			{0.7, 0.3, 0}, // doc2 中等相似
		},
	}

	// 仅返回前 2 条.
	reranker := NewEmbeddingReranker(embed, WithTopN(2))
	result, err := reranker.Rerank(t.Context(), "查询", docs)
	if err != nil {
		t.Fatalf("Rerank 失败: %v", err)
	}

	// 验证仅返回 2 条.
	if len(result) != 2 {
		t.Fatalf("期望 2 条结果，实际 %d 条", len(result))
	}

	// 验证前两名为 doc0 和 doc2.
	if result[0].ID != "doc0" {
		t.Errorf("期望 result[0]=doc0，实际=%s", result[0].ID)
	}
	if result[1].ID != "doc2" {
		t.Errorf("期望 result[1]=doc2，实际=%s", result[1].ID)
	}
}

// ──────────────────────────────────────────
// TestLLMReranker_NilModel
// ──────────────────────────────────────────

// TestLLMReranker_NilModel 验证传入 nil 模型时返回 ErrNilModel.
func TestLLMReranker_NilModel(t *testing.T) {
	reranker := NewLLMReranker(nil)
	_, err := reranker.Rerank(t.Context(), "查询", makeDocs("文档A"))
	if err != ErrNilModel {
		t.Fatalf("期望 ErrNilModel，实际: %v", err)
	}
}

// ──────────────────────────────────────────
// TestLLMReranker_EmptyDocs
// ──────────────────────────────────────────

// TestLLMReranker_EmptyDocs 验证传入空文档列表时返回 ErrEmptyDocs.
func TestLLMReranker_EmptyDocs(t *testing.T) {
	reranker := NewLLMReranker(&mockChat{})
	_, err := reranker.Rerank(t.Context(), "查询", nil)
	if err != ErrEmptyDocs {
		t.Fatalf("期望 ErrEmptyDocs，实际: %v", err)
	}
}

// ──────────────────────────────────────────
// TestCrossEncoderReranker
// ──────────────────────────────────────────

// TestCrossEncoderReranker 验证 CrossEncoderReranker 能够正确调用外部 API 并重排序文档.
func TestCrossEncoderReranker(t *testing.T) {
	docs := makeDocs("文档A", "文档B", "文档C")

	// 创建模拟 HTTP 服务器，返回预设评分结果.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求方法和 Content-Type.
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST 请求，实际 %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("期望 Content-Type=application/json，实际=%s", r.Header.Get("Content-Type"))
		}

		// 返回 doc1(score=0.9) > doc2(score=0.7) > doc0(score=0.3).
		resp := crossEncoderResponse{
			Results: []crossEncoderResultItem{
				{Index: 1, RelevanceScore: 0.9},
				{Index: 2, RelevanceScore: 0.7},
				{Index: 0, RelevanceScore: 0.3},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	reranker := NewCrossEncoderReranker(server.URL)
	result, err := reranker.Rerank(t.Context(), "测试查询", docs)
	if err != nil {
		t.Fatalf("Rerank 失败: %v", err)
	}

	// 验证返回数量.
	if len(result) != 3 {
		t.Fatalf("期望 3 条结果，实际 %d 条", len(result))
	}

	// 验证排序：doc1(0.9) > doc2(0.7) > doc0(0.3).
	expectedOrder := []string{"doc1", "doc2", "doc0"}
	for i, expected := range expectedOrder {
		if result[i].ID != expected {
			t.Errorf("位置 %d：期望 %s，实际 %s", i, expected, result[i].ID)
		}
	}
}

// TestCrossEncoderReranker_EmptyEndpoint 验证端点为空时返回 ErrEmptyEndpoint.
func TestCrossEncoderReranker_EmptyEndpoint(t *testing.T) {
	reranker := NewCrossEncoderReranker("")
	_, err := reranker.Rerank(t.Context(), "查询", makeDocs("文档A"))
	if err != ErrEmptyEndpoint {
		t.Fatalf("期望 ErrEmptyEndpoint，实际: %v", err)
	}
}
