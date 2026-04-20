package hybrid

import (
	"reflect"
	"testing"

	"github.com/Tsukikage7/servex/v2/llm/retrieval/rag"
)

// ──────────────────────────────────────────
// TestTokenize
// ──────────────────────────────────────────

// TestTokenize 验证分词：ASCII 字母/数字累积成一 token，CJK 每字单独成 token，
// 其他字符作分隔符.
func TestTokenize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "英文小写",
			in:   "hello world",
			want: []string{"hello", "world"},
		},
		{
			name: "英文混合大小写",
			in:   "Hello World",
			want: []string{"hello", "world"},
		},
		{
			name: "中英数字混合",
			in:   "Hello 世界 123!",
			want: []string{"hello", "世", "界", "123"},
		},
		{
			name: "下划线作分隔",
			in:   "VPS_Server",
			want: []string{"vps", "server"},
		},
		{
			name: "标点作分隔",
			in:   "a,b;c.d",
			want: []string{"a", "b", "c", "d"},
		},
		{
			name: "纯中文",
			in:   "退款政策",
			want: []string{"退", "款", "政", "策"},
		},
		{
			name: "空字符串",
			in:   "",
			want: nil,
		},
		{
			name: "仅空白",
			in:   "   \t\n",
			want: nil,
		},
		{
			name: "字母紧接数字",
			in:   "abc123",
			want: []string{"abc123"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tokenize(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("tokenize(%q) = %v，期望 %v", tc.in, got, tc.want)
			}
		})
	}
}

// ──────────────────────────────────────────
// TestBM25Retriever_Basic
// ──────────────────────────────────────────

// TestBM25Retriever_Basic 验证基础命中：文档含 query 词 → score > 0.
func TestBM25Retriever_Basic(t *testing.T) {
	docs := []rag.Document{
		{ID: "d1", Content: "golang is a compiled language"},
		{ID: "d2", Content: "python is an interpreted language"},
		{ID: "d3", Content: "rust is a systems language"},
	}
	r := NewBM25Retriever(docs)

	result, err := r.Retrieve(t.Context(), "golang", 5)
	if err != nil {
		t.Fatalf("Retrieve 失败: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("期望至少 1 条结果")
	}
	if result[0].ID != "d1" {
		t.Errorf("期望 d1 首位，实际 %s", result[0].ID)
	}
	if result[0].Score <= 0 {
		t.Errorf("期望 score > 0，实际 %f", result[0].Score)
	}
}

// ──────────────────────────────────────────
// TestBM25Retriever_TermFrequency
// ──────────────────────────────────────────

// TestBM25Retriever_TermFrequency 验证词频越高排序越靠前（在长度相近时）.
func TestBM25Retriever_TermFrequency(t *testing.T) {
	// d1 出现 "退款" 3 次；d2 出现 1 次；两者长度相近.
	docs := []rag.Document{
		{ID: "d1", Content: "退款 退款 退款 政策 说明 文档 详细"},
		{ID: "d2", Content: "退款 政策 说明 文档 详细 介绍 内容"},
		{ID: "d3", Content: "订单 发货 物流 查询 状态 变更 信息"},
	}
	r := NewBM25Retriever(docs)

	result, err := r.Retrieve(t.Context(), "退款", 3)
	if err != nil {
		t.Fatalf("Retrieve 失败: %v", err)
	}
	if len(result) < 2 {
		t.Fatalf("期望至少 2 条结果，实际 %d 条", len(result))
	}
	if result[0].ID != "d1" || result[1].ID != "d2" {
		t.Errorf("期望排序 d1, d2；实际 %s, %s", result[0].ID, result[1].ID)
	}
	if result[0].Score <= result[1].Score {
		t.Errorf("d1 分数应大于 d2：%f vs %f", result[0].Score, result[1].Score)
	}
}

// ──────────────────────────────────────────
// TestBM25Retriever_LengthPenalty
// ──────────────────────────────────────────

// TestBM25Retriever_LengthPenalty 验证长度归一化：相同命中词，短文档得分更高.
func TestBM25Retriever_LengthPenalty(t *testing.T) {
	// d1 短文档；d2 长文档；均只出现 1 次 "退款".
	docs := []rag.Document{
		{ID: "d1", Content: "退款 政策"},
		{ID: "d2", Content: "退款 政策 其他 很多 无关 内容 填充 长度 惩罚 测试 数据 多余 段落"},
		// 再加一个干扰文档以让 avgdl 有意义.
		{ID: "d3", Content: "发货 物流"},
	}
	r := NewBM25Retriever(docs)

	result, err := r.Retrieve(t.Context(), "退款", 5)
	if err != nil {
		t.Fatalf("Retrieve 失败: %v", err)
	}
	if len(result) < 2 {
		t.Fatalf("期望至少 2 条结果")
	}
	if result[0].ID != "d1" {
		t.Errorf("期望短文档 d1 首位，实际 %s", result[0].ID)
	}
}

// ──────────────────────────────────────────
// TestBM25Retriever_Chinese
// ──────────────────────────────────────────

// TestBM25Retriever_Chinese 验证中文单字分词：query "退款政策" 命中含 "退款" 的文档.
func TestBM25Retriever_Chinese(t *testing.T) {
	docs := []rag.Document{
		{ID: "d1", Content: "关于退款的详细说明"},
		{ID: "d2", Content: "订单查询与发货信息"},
		{ID: "d3", Content: "退款流程与政策解读"},
	}
	r := NewBM25Retriever(docs)

	result, err := r.Retrieve(t.Context(), "退款政策", 3)
	if err != nil {
		t.Fatalf("Retrieve 失败: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("期望至少 1 条结果")
	}
	// d3 同时含 "退"、"款"、"政"、"策"；d1 含 "退"、"款"；d2 都不命中.
	if result[0].ID != "d3" {
		t.Errorf("期望 d3 首位，实际 %s", result[0].ID)
	}
	// d2 不应命中.
	for _, d := range result {
		if d.ID == "d2" {
			t.Errorf("d2 不应命中，实际命中 score=%f", d.Score)
		}
	}
}

// ──────────────────────────────────────────
// TestBM25Retriever_EmptyCorpus
// ──────────────────────────────────────────

// TestBM25Retriever_EmptyCorpus 验证空语料返回空结果.
func TestBM25Retriever_EmptyCorpus(t *testing.T) {
	r := NewBM25Retriever(nil)
	result, err := r.Retrieve(t.Context(), "query", 5)
	if err != nil {
		t.Fatalf("期望 nil 错误，实际: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("期望空结果，实际 %d 条", len(result))
	}
}

// ──────────────────────────────────────────
// TestBM25Retriever_EmptyQuery
// ──────────────────────────────────────────

// TestBM25Retriever_EmptyQuery 验证空查询返回空结果.
func TestBM25Retriever_EmptyQuery(t *testing.T) {
	docs := []rag.Document{
		{ID: "d1", Content: "some content"},
	}
	r := NewBM25Retriever(docs)
	result, err := r.Retrieve(t.Context(), "", 5)
	if err != nil {
		t.Fatalf("期望 nil 错误，实际: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("期望空结果，实际 %d 条", len(result))
	}
}

// ──────────────────────────────────────────
// TestBM25Retriever_TopK
// ──────────────────────────────────────────

// TestBM25Retriever_TopK 验证 topK 截断.
func TestBM25Retriever_TopK(t *testing.T) {
	docs := []rag.Document{
		{ID: "d1", Content: "退款"},
		{ID: "d2", Content: "退款"},
		{ID: "d3", Content: "退款"},
		{ID: "d4", Content: "退款"},
	}
	r := NewBM25Retriever(docs)
	result, err := r.Retrieve(t.Context(), "退款", 2)
	if err != nil {
		t.Fatalf("Retrieve 失败: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("期望 2 条结果，实际 %d 条", len(result))
	}
}

// ──────────────────────────────────────────
// TestBM25Retriever_CustomParams
// ──────────────────────────────────────────

// TestBM25Retriever_CustomParams 验证自定义 k1/b 参数能正常工作.
func TestBM25Retriever_CustomParams(t *testing.T) {
	docs := []rag.Document{
		{ID: "d1", Content: "退款 政策"},
		{ID: "d2", Content: "发货 物流"},
	}
	r := NewBM25Retriever(docs, WithBM25Params(1.2, 0.5))
	result, err := r.Retrieve(t.Context(), "退款", 5)
	if err != nil {
		t.Fatalf("Retrieve 失败: %v", err)
	}
	if len(result) == 0 || result[0].ID != "d1" {
		t.Errorf("期望 d1 首位，实际 %+v", result)
	}
}
