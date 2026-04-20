package rerank

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

// ──────────────────────────────────────────
// 扩展 mock:支持错误注入与批次计数
// ──────────────────────────────────────────

// mockChatErr 返回指定错误的 ChatModel mock.
type mockChatErr struct {
	err error
}

func (m *mockChatErr) Generate(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
	return nil, m.err
}

func (m *mockChatErr) Stream(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (llm.StreamReader, error) {
	return nil, m.err
}

// mockChatBatch 支持记录每次批调用的 prompt 并按调用序号返回不同 JSON.
type mockChatBatch struct {
	responses []string
	calls     atomic.Int32
	prompts   []string
}

func (m *mockChatBatch) Generate(_ context.Context, msgs []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
	idx := int(m.calls.Add(1)) - 1
	// 记录 prompt，便于验证 offset.
	if len(msgs) > 0 {
		m.prompts = append(m.prompts, msgs[0].Content)
	}
	content := ""
	if idx < len(m.responses) {
		content = m.responses[idx]
	}
	return &llm.ChatResponse{Message: llm.AssistantMessage(content)}, nil
}

func (m *mockChatBatch) Stream(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (llm.StreamReader, error) {
	return nil, fmt.Errorf("mockChatBatch: 不支持 Stream")
}

// mockEmbeddingErr 返回错误的 EmbeddingModel mock.
type mockEmbeddingErr struct {
	err error
}

func (m *mockEmbeddingErr) EmbedTexts(_ context.Context, _ []string, _ ...llm.CallOption) (*llm.EmbedResponse, error) {
	return nil, m.err
}

// mockEmbeddingShort 返回少于请求数的向量，用于触发数量不足分支.
type mockEmbeddingShort struct{}

func (m *mockEmbeddingShort) EmbedTexts(_ context.Context, texts []string, _ ...llm.CallOption) (*llm.EmbedResponse, error) {
	// 故意只返回 len(texts)-1 条。
	n := len(texts) - 1
	if n < 0 {
		n = 0
	}
	out := make([][]float32, n)
	for i := range out {
		out[i] = []float32{1, 0, 0}
	}
	return &llm.EmbedResponse{Embeddings: out, ModelID: "mock-short"}, nil
}

// ──────────────────────────────────────────
// TestLLMReranker_BatchProcessing
// ──────────────────────────────────────────

// TestLLMReranker_BatchProcessing 验证 batchSize < 文档数时，
// 多次调用 LLM 并正确合并各批次评分（offset 正确传递）。
func TestLLMReranker_BatchProcessing(t *testing.T) {
	docs := makeDocs("A", "B", "C", "D", "E")

	// batchSize = 2 -> 3 个批次：[0,1]、[2,3]、[4]
	// 每批各返回对应全局索引的评分。
	chat := &mockChatBatch{
		responses: []string{
			`[{"index":0,"score":1},{"index":1,"score":5}]`,
			`[{"index":2,"score":9},{"index":3,"score":3}]`,
			`[{"index":4,"score":7}]`,
		},
	}

	reranker := NewLLMReranker(chat, WithBatchSize(2))
	result, err := reranker.Rerank(t.Context(), "查询", docs)
	if err != nil {
		t.Fatalf("Rerank 失败: %v", err)
	}

	// 应调用 3 次。
	if got := chat.calls.Load(); got != 3 {
		t.Fatalf("期望 Generate 调用 3 次，实际 %d 次", got)
	}

	// 期望排序：doc2(9) > doc4(7) > doc1(5) > doc3(3) > doc0(1)。
	want := []string{"doc2", "doc4", "doc1", "doc3", "doc0"}
	if len(result) != len(want) {
		t.Fatalf("期望 %d 条结果，实际 %d 条", len(want), len(result))
	}
	for i, w := range want {
		if result[i].ID != w {
			t.Errorf("位置 %d：期望 %s，实际 %s（Score=%f）", i, w, result[i].ID, result[i].Score)
		}
	}
}

// TestLLMReranker_BatchSizeOne 验证 batchSize=1 的极端边界，每篇文档一批。
func TestLLMReranker_BatchSizeOne(t *testing.T) {
	docs := makeDocs("A", "B", "C")

	chat := &mockChatBatch{
		responses: []string{
			`[{"index":0,"score":2}]`,
			`[{"index":1,"score":8}]`,
			`[{"index":2,"score":5}]`,
		},
	}

	reranker := NewLLMReranker(chat, WithBatchSize(1))
	result, err := reranker.Rerank(t.Context(), "查询", docs)
	if err != nil {
		t.Fatalf("Rerank 失败: %v", err)
	}

	if got := chat.calls.Load(); got != 3 {
		t.Fatalf("期望 Generate 调用 3 次，实际 %d 次", got)
	}

	// doc1(8) > doc2(5) > doc0(2)
	want := []string{"doc1", "doc2", "doc0"}
	for i, w := range want {
		if result[i].ID != w {
			t.Errorf("位置 %d：期望 %s，实际 %s", i, w, result[i].ID)
		}
	}
}

// TestLLMReranker_LLMError 验证 LLM 调用出错时 Rerank 返回包装错误，
// 且保留原始错误（使用 errors.Is 可追溯）。
func TestLLMReranker_LLMError(t *testing.T) {
	sentinel := errors.New("llm boom")
	reranker := NewLLMReranker(&mockChatErr{err: sentinel})
	_, err := reranker.Rerank(t.Context(), "查询", makeDocs("A", "B"))
	if err == nil {
		t.Fatalf("期望返回错误，实际为 nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("期望错误包含 %v，实际=%v", sentinel, err)
	}
}

// ──────────────────────────────────────────
// TestLLMReranker_ParseErrors
// ──────────────────────────────────────────

// TestLLMReranker_ParseErrors 表格化覆盖 parseLLMScores 的异常输入。
// 黑盒方式：通过 mock LLM 返回指定字符串，断言 Rerank 的输出/错误。
func TestLLMReranker_ParseErrors(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantErr   bool
		// orderCheck 若非空，验证结果排序。
		orderCheck []string
	}{
		{
			name:    "非 JSON 纯文本",
			content: "抱歉，我无法评分。",
			wantErr: true,
		},
		{
			name:    "缺少数组分隔符",
			content: `{"index":0,"score":1}`,
			wantErr: true,
		},
		{
			name:    "JSON 数组语法错误",
			content: `[{"index":0,"score":]`,
			wantErr: true,
		},
		{
			name:       "缺 score 字段 -> Score 为 0",
			content:    `[{"index":0},{"index":1,"score":5},{"index":2,"score":3}]`,
			wantErr:    false,
			orderCheck: []string{"doc1", "doc2", "doc0"},
		},
		{
			name:       "index 越界被忽略",
			content:    `[{"index":99,"score":100},{"index":1,"score":5},{"index":0,"score":2}]`,
			wantErr:    false,
			orderCheck: []string{"doc1", "doc0", "doc2"},
		},
		{
			name:    "含代码块标记仍可提取 JSON",
			content: "```json\n[{\"index\":0,\"score\":9},{\"index\":1,\"score\":1},{\"index\":2,\"score\":0}]\n```",
			wantErr: false,
			// doc0(9) > doc1(1) > doc2(0)
			orderCheck: []string{"doc0", "doc1", "doc2"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			docs := makeDocs("A", "B", "C")
			chat := &mockChat{fn: func(_ []llm.Message) string { return tc.content }}
			result, err := NewLLMReranker(chat).Rerank(t.Context(), "q", docs)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("期望返回错误，实际为 nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("不期望错误，实际=%v", err)
			}
			if len(tc.orderCheck) > 0 {
				if len(result) != len(tc.orderCheck) {
					t.Fatalf("期望 %d 条，实际 %d 条", len(tc.orderCheck), len(result))
				}
				for i, w := range tc.orderCheck {
					if result[i].ID != w {
						t.Errorf("位置 %d：期望 %s，实际 %s", i, w, result[i].ID)
					}
				}
			}
		})
	}
}

// ──────────────────────────────────────────
// TestEmbeddingReranker 错误路径与边界
// ──────────────────────────────────────────

// TestEmbeddingReranker_NilModel 验证 nil 模型返回 ErrNilModel。
func TestEmbeddingReranker_NilModel(t *testing.T) {
	reranker := NewEmbeddingReranker(nil)
	_, err := reranker.Rerank(t.Context(), "q", makeDocs("A"))
	if err != ErrNilModel {
		t.Fatalf("期望 ErrNilModel，实际=%v", err)
	}
}

// TestEmbeddingReranker_EmptyDocs 验证空文档返回 ErrEmptyDocs。
func TestEmbeddingReranker_EmptyDocs(t *testing.T) {
	reranker := NewEmbeddingReranker(&mockEmbedding{})
	_, err := reranker.Rerank(t.Context(), "q", nil)
	if err != ErrEmptyDocs {
		t.Fatalf("期望 ErrEmptyDocs，实际=%v", err)
	}
}

// TestEmbeddingReranker_EmbedError 验证 embedder 报错时 Rerank 返回包装错误。
func TestEmbeddingReranker_EmbedError(t *testing.T) {
	sentinel := errors.New("embed boom")
	reranker := NewEmbeddingReranker(&mockEmbeddingErr{err: sentinel})
	_, err := reranker.Rerank(t.Context(), "q", makeDocs("A"))
	if err == nil {
		t.Fatalf("期望错误，实际为 nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("期望错误包含 %v，实际=%v", sentinel, err)
	}
}

// TestEmbeddingReranker_ShortEmbeddings 验证返回向量数量少于 len(docs)+1 时报错。
func TestEmbeddingReranker_ShortEmbeddings(t *testing.T) {
	reranker := NewEmbeddingReranker(&mockEmbeddingShort{})
	_, err := reranker.Rerank(t.Context(), "q", makeDocs("A", "B"))
	if err == nil {
		t.Fatalf("期望错误，实际为 nil")
	}
	if !strings.Contains(err.Error(), "向量数量不足") {
		t.Errorf("错误信息未提示数量不足：%v", err)
	}
}

// TestEmbeddingReranker_ZeroVectors 验证全零向量不会 panic/NaN，
// 并且最终结果长度与输入一致（余弦分数可能为 0/NaN，但不应崩溃）。
func TestEmbeddingReranker_ZeroVectors(t *testing.T) {
	docs := makeDocs("A", "B")
	embed := &mockEmbedding{
		embeddings: [][]float32{
			{0, 0, 0}, // query
			{0, 0, 0}, // doc0
			{0, 0, 0}, // doc1
		},
	}
	reranker := NewEmbeddingReranker(embed)
	result, err := reranker.Rerank(t.Context(), "q", docs)
	if err != nil {
		t.Fatalf("不期望错误：%v", err)
	}
	if len(result) != 2 {
		t.Fatalf("期望 2 条结果，实际 %d", len(result))
	}
}

// ──────────────────────────────────────────
// TestCrossEncoderReranker 扩展测试
// ──────────────────────────────────────────

// TestCrossEncoderReranker_EmptyDocs 验证空文档返回 ErrEmptyDocs（不发起请求）。
func TestCrossEncoderReranker_EmptyDocs(t *testing.T) {
	// 使用一个不应被访问的端点；若发起请求则测试会通过但说明行为错误。
	reranker := NewCrossEncoderReranker("http://127.0.0.1:0")
	_, err := reranker.Rerank(t.Context(), "q", nil)
	if err != ErrEmptyDocs {
		t.Fatalf("期望 ErrEmptyDocs，实际=%v", err)
	}
}

// TestCrossEncoderReranker_HTTP500 验证 5xx 状态码返回 ErrAPIFailed 包装。
func TestCrossEncoderReranker_HTTP500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	reranker := NewCrossEncoderReranker(server.URL)
	_, err := reranker.Rerank(t.Context(), "q", makeDocs("A"))
	if err == nil {
		t.Fatalf("期望错误，实际为 nil")
	}
	if !errors.Is(err, ErrAPIFailed) {
		t.Errorf("期望错误包含 ErrAPIFailed，实际=%v", err)
	}
}

// TestCrossEncoderReranker_MalformedJSON 验证 200 但响应体无法解析时返回错误。
func TestCrossEncoderReranker_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not-json`))
	}))
	defer server.Close()

	reranker := NewCrossEncoderReranker(server.URL)
	_, err := reranker.Rerank(t.Context(), "q", makeDocs("A"))
	if err == nil {
		t.Fatalf("期望错误，实际为 nil")
	}
	if !strings.Contains(err.Error(), "解析 API 响应失败") {
		t.Errorf("期望解析失败错误，实际=%v", err)
	}
}

// TestCrossEncoderReranker_PartialResults 验证 API 返回 results 数量少于输入 docs 时，
// 缺失的文档按原顺序追加到末尾（覆盖 appendUnranked 路径）。
func TestCrossEncoderReranker_PartialResults(t *testing.T) {
	docs := makeDocs("A", "B", "C", "D")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// 仅返回 doc2 和 doc0 的评分，doc1、doc3 缺失。
		resp := crossEncoderResponse{
			Results: []crossEncoderResultItem{
				{Index: 2, RelevanceScore: 0.8},
				{Index: 0, RelevanceScore: 0.4},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	reranker := NewCrossEncoderReranker(server.URL)
	result, err := reranker.Rerank(t.Context(), "q", docs)
	if err != nil {
		t.Fatalf("Rerank 失败: %v", err)
	}

	// 前两条为 ranked（doc2 > doc0），后两条为未评分的 doc1 / doc3，
	// 原顺序（doc1 在 doc3 前）。
	want := []string{"doc2", "doc0", "doc1", "doc3"}
	if len(result) != len(want) {
		t.Fatalf("期望 %d 条，实际 %d 条", len(want), len(result))
	}
	for i, w := range want {
		if result[i].ID != w {
			t.Errorf("位置 %d：期望 %s，实际 %s", i, w, result[i].ID)
		}
	}
}

// TestCrossEncoderReranker_WithAPIKey 验证 WithAPIKey 正确设置 Authorization 头。
func TestCrossEncoderReranker_WithAPIKey(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		resp := crossEncoderResponse{
			Results: []crossEncoderResultItem{{Index: 0, RelevanceScore: 1}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	reranker := NewCrossEncoderReranker(server.URL, WithAPIKey("secret-key"))
	_, err := reranker.Rerank(t.Context(), "q", makeDocs("A"))
	if err != nil {
		t.Fatalf("Rerank 失败: %v", err)
	}
	if gotAuth != "Bearer secret-key" {
		t.Errorf("期望 Authorization=Bearer secret-key，实际=%q", gotAuth)
	}
}

// TestCrossEncoderReranker_WithModel 验证 WithModel 将 model 写入请求体。
func TestCrossEncoderReranker_WithModel(t *testing.T) {
	var gotBody crossEncoderRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		resp := crossEncoderResponse{
			Results: []crossEncoderResultItem{{Index: 0, RelevanceScore: 1}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	reranker := NewCrossEncoderReranker(server.URL, WithModel("bge-reranker-v2"))
	_, err := reranker.Rerank(t.Context(), "查询", makeDocs("A", "B"))
	if err != nil {
		t.Fatalf("Rerank 失败: %v", err)
	}
	if gotBody.Model != "bge-reranker-v2" {
		t.Errorf("期望请求体 model=bge-reranker-v2，实际=%q", gotBody.Model)
	}
	if gotBody.Query != "查询" {
		t.Errorf("期望 query=查询，实际=%q", gotBody.Query)
	}
	if len(gotBody.Documents) != 2 {
		t.Errorf("期望 documents 长度 2，实际 %d", len(gotBody.Documents))
	}
}

// TestCrossEncoderReranker_CtxCanceled 验证 ctx 取消时请求出错（触发 ErrAPIFailed 路径）。
func TestCrossEncoderReranker_CtxCanceled(t *testing.T) {
	// 使用一个永远阻塞的服务端，不过测试中我们会立即取消 ctx。
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	reranker := NewCrossEncoderReranker(server.URL)
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // 立即取消

	_, err := reranker.Rerank(ctx, "q", makeDocs("A"))
	if err == nil {
		t.Fatalf("期望错误，实际为 nil")
	}
	if !errors.Is(err, ErrAPIFailed) && !errors.Is(err, context.Canceled) {
		t.Errorf("期望错误关联 ErrAPIFailed 或 context.Canceled，实际=%v", err)
	}
}

// ──────────────────────────────────────────
// TestApplyTopN
// ──────────────────────────────────────────

// TestApplyTopN 表格化覆盖 applyTopN 的三种边界：
// topN=0 不截断、topN 大于长度不截断、topN 小于长度正确截断。
func TestApplyTopN(t *testing.T) {
	docs := makeDocs("A", "B", "C")
	tests := []struct {
		name    string
		topN    int
		wantLen int
	}{
		{"topN=0 不截断", 0, 3},
		{"topN=-1 不截断", -1, 3},
		{"topN 大于长度不截断", 10, 3},
		{"topN 等于长度不截断", 3, 3},
		{"topN 小于长度截断", 2, 2},
		{"topN=1 截断到 1 条", 1, 1},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := applyTopN(docs, tc.topN)
			if len(got) != tc.wantLen {
				t.Errorf("topN=%d，期望长度 %d，实际 %d", tc.topN, tc.wantLen, len(got))
			}
		})
	}
}
