package hybrid

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/Tsukikage7/servex/v2/llm/retrieval/rag"
)

// ──────────────────────────────────────────
// Mock Retriever
// ──────────────────────────────────────────

// mockRetriever 按预设结果返回，用于测试融合逻辑.
type mockRetriever struct {
	// docs 预设的检索结果.
	docs []rag.RetrievedDoc
	// err 预设的错误.
	err error
}

// Retrieve 实现 Retriever 接口，返回预设的 docs/err.
func (m *mockRetriever) Retrieve(_ context.Context, _ string, _ int) ([]rag.RetrievedDoc, error) {
	return m.docs, m.err
}

// ──────────────────────────────────────────
// 辅助函数
// ──────────────────────────────────────────

// makeDocs 根据 ids 构造按给定顺序排列的 RetrievedDoc 列表.
// Score 置为 1.0（RRF 融合时原始 Score 不参与计算）.
func makeDocs(ids ...string) []rag.RetrievedDoc {
	docs := make([]rag.RetrievedDoc, len(ids))
	for i, id := range ids {
		docs[i] = rag.RetrievedDoc{
			Document: rag.Document{ID: id, Content: id + "-content"},
			Score:    1.0,
		}
	}
	return docs
}

// findDoc 在融合结果中按 ID 查找文档，未找到返回 nil.
func findDoc(result []rag.RetrievedDoc, id string) *rag.RetrievedDoc {
	for i := range result {
		if result[i].ID == id {
			return &result[i]
		}
	}
	return nil
}

// floatEq 浮点近似相等判定，容差放宽至 1e-5 以避免 float32 精度累积 flaky.
func floatEq(a, b float32) bool {
	return math.Abs(float64(a-b)) < 1e-5
}

// ──────────────────────────────────────────
// TestHybridRetriever_RRFFormula
// ──────────────────────────────────────────

// TestHybridRetriever_RRFFormula 验证 RRF 融合公式：
// 两路均返回独立 ID 的文档，期望分数严格等于 weight/(k + rank).
func TestHybridRetriever_RRFFormula(t *testing.T) {
	// 向量路：vecA(rank0), vecB(rank1).
	vec := &mockRetriever{docs: makeDocs("vecA", "vecB")}
	// 词法路：lexA(rank0), lexB(rank1).
	lex := &mockRetriever{docs: makeDocs("lexA", "lexB")}

	h := New(vec, lex, WithRRFK(60), WithWeights(1.0, 1.0))
	result, err := h.Retrieve(t.Context(), "query", 10)
	if err != nil {
		t.Fatalf("Retrieve 失败: %v", err)
	}
	if len(result) != 4 {
		t.Fatalf("期望 4 条结果，实际 %d 条", len(result))
	}

	// 预期 score：rank0 → 1/(60+1) ≈ 0.01639; rank1 → 1/(60+2) ≈ 0.01613.
	const expectRank0 = 1.0 / (60.0 + 1.0)
	const expectRank1 = 1.0 / (60.0 + 2.0)

	for _, id := range []string{"vecA", "lexA"} {
		d := findDoc(result, id)
		if d == nil {
			t.Fatalf("结果中未找到 %s", id)
		}
		if !floatEq(d.Score, float32(expectRank0)) {
			t.Errorf("%s 分数不符：期望 %f，实际 %f", id, expectRank0, d.Score)
		}
	}
	for _, id := range []string{"vecB", "lexB"} {
		d := findDoc(result, id)
		if d == nil {
			t.Fatalf("结果中未找到 %s", id)
		}
		if !floatEq(d.Score, float32(expectRank1)) {
			t.Errorf("%s 分数不符：期望 %f，实际 %f", id, expectRank1, d.Score)
		}
	}
}

// ──────────────────────────────────────────
// TestHybridRetriever_Overlap
// ──────────────────────────────────────────

// TestHybridRetriever_Overlap 验证同一 ID 出现在两路时分数累加.
func TestHybridRetriever_Overlap(t *testing.T) {
	// docX 在两路均 rank=0；docY 只在向量 rank=1；docZ 只在词法 rank=1.
	vec := &mockRetriever{docs: makeDocs("docX", "docY")}
	lex := &mockRetriever{docs: makeDocs("docX", "docZ")}

	h := New(vec, lex, WithRRFK(60), WithWeights(1.0, 1.0))
	result, err := h.Retrieve(t.Context(), "query", 10)
	if err != nil {
		t.Fatalf("Retrieve 失败: %v", err)
	}

	// docX 命中两路，score = 1/(60+1) + 1/(60+1).
	docX := findDoc(result, "docX")
	if docX == nil {
		t.Fatal("结果中未找到 docX")
	}
	const expectX = 2.0 / (60.0 + 1.0)
	if !floatEq(docX.Score, float32(expectX)) {
		t.Errorf("docX 分数不符：期望 %f，实际 %f", expectX, docX.Score)
	}

	// docY / docZ 单路命中 rank=1.
	const expectOne = 1.0 / (60.0 + 2.0)
	for _, id := range []string{"docY", "docZ"} {
		d := findDoc(result, id)
		if d == nil {
			t.Fatalf("结果中未找到 %s", id)
		}
		if !floatEq(d.Score, float32(expectOne)) {
			t.Errorf("%s 分数不符：期望 %f，实际 %f", id, expectOne, d.Score)
		}
	}

	// 按 Score 降序：docX 排在最前.
	if result[0].ID != "docX" {
		t.Errorf("期望首位为 docX，实际 %s", result[0].ID)
	}
}

// ──────────────────────────────────────────
// TestHybridRetriever_Weights
// ──────────────────────────────────────────

// TestHybridRetriever_Weights 验证权重生效：向量权重更高时，向量命中分数更大.
func TestHybridRetriever_Weights(t *testing.T) {
	vec := &mockRetriever{docs: makeDocs("vecHit")}
	lex := &mockRetriever{docs: makeDocs("lexHit")}

	h := New(vec, lex, WithRRFK(60), WithWeights(2.0, 1.0))
	result, err := h.Retrieve(t.Context(), "query", 10)
	if err != nil {
		t.Fatalf("Retrieve 失败: %v", err)
	}

	vd := findDoc(result, "vecHit")
	ld := findDoc(result, "lexHit")
	if vd == nil || ld == nil {
		t.Fatalf("结果缺失：vecHit=%v lexHit=%v", vd, ld)
	}

	const expectVec = 2.0 / (60.0 + 1.0)
	const expectLex = 1.0 / (60.0 + 1.0)
	if !floatEq(vd.Score, float32(expectVec)) {
		t.Errorf("vecHit 分数不符：期望 %f，实际 %f", expectVec, vd.Score)
	}
	if !floatEq(ld.Score, float32(expectLex)) {
		t.Errorf("lexHit 分数不符：期望 %f，实际 %f", expectLex, ld.Score)
	}
	if result[0].ID != "vecHit" {
		t.Errorf("高权重路文档应当排在首位，实际 %s", result[0].ID)
	}
}

// ──────────────────────────────────────────
// TestHybridRetriever_TopKTruncate
// ──────────────────────────────────────────

// TestHybridRetriever_TopKTruncate 验证 topK 截断.
func TestHybridRetriever_TopKTruncate(t *testing.T) {
	vec := &mockRetriever{docs: makeDocs("a", "b", "c", "d", "e")}
	lex := &mockRetriever{docs: makeDocs("f", "g", "h", "i", "j")}

	h := New(vec, lex)
	result, err := h.Retrieve(t.Context(), "q", 3)
	if err != nil {
		t.Fatalf("Retrieve 失败: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("期望 3 条结果，实际 %d 条", len(result))
	}
}

// ──────────────────────────────────────────
// TestHybridRetriever_PartialFailure
// ──────────────────────────────────────────

// TestHybridRetriever_PartialFailure 验证一路失败仍返回另一路结果，不报错.
func TestHybridRetriever_PartialFailure(t *testing.T) {
	errVec := errors.New("vector store unavailable")
	vec := &mockRetriever{err: errVec}
	lex := &mockRetriever{docs: makeDocs("lexA", "lexB")}

	h := New(vec, lex)
	result, err := h.Retrieve(t.Context(), "q", 10)
	if err != nil {
		t.Fatalf("期望 nil 错误，实际: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("期望 2 条结果，实际 %d 条", len(result))
	}
	if findDoc(result, "lexA") == nil || findDoc(result, "lexB") == nil {
		t.Errorf("应当返回 lexA、lexB，实际 %+v", result)
	}
}

// ──────────────────────────────────────────
// TestHybridRetriever_BothFail
// ──────────────────────────────────────────

// TestHybridRetriever_BothFail 验证两路都失败时返回 joined error.
func TestHybridRetriever_BothFail(t *testing.T) {
	errVec := errors.New("vec fail")
	errLex := errors.New("lex fail")
	vec := &mockRetriever{err: errVec}
	lex := &mockRetriever{err: errLex}

	h := New(vec, lex)
	_, err := h.Retrieve(t.Context(), "q", 10)
	if err == nil {
		t.Fatal("期望错误，实际 nil")
	}
	if !errors.Is(err, errVec) {
		t.Errorf("期望 err 包含 errVec")
	}
	if !errors.Is(err, errLex) {
		t.Errorf("期望 err 包含 errLex")
	}
}

// ──────────────────────────────────────────
// TestHybridRetriever_EmptyResults
// ──────────────────────────────────────────

// TestHybridRetriever_EmptyResults 验证两路都返回空时返回空 slice 且无错误.
func TestHybridRetriever_EmptyResults(t *testing.T) {
	vec := &mockRetriever{docs: nil}
	lex := &mockRetriever{docs: nil}

	h := New(vec, lex)
	result, err := h.Retrieve(t.Context(), "q", 10)
	if err != nil {
		t.Fatalf("期望 nil 错误，实际: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("期望空结果，实际 %d 条", len(result))
	}
}

// ──────────────────────────────────────────
// TestHybridRetriever_DefaultOptions
// ──────────────────────────────────────────

// TestHybridRetriever_DefaultOptions 验证默认参数：k=60，权重均为 1.0.
func TestHybridRetriever_DefaultOptions(t *testing.T) {
	vec := &mockRetriever{docs: makeDocs("docA")}
	lex := &mockRetriever{docs: makeDocs("docB")}

	h := New(vec, lex) // 不传 opts
	result, err := h.Retrieve(t.Context(), "q", 10)
	if err != nil {
		t.Fatalf("Retrieve 失败: %v", err)
	}

	// 默认 k=60, 权重 1.0 → 每个文档 rank=0 score = 1/(60+1).
	const expect = 1.0 / (60.0 + 1.0)
	for _, id := range []string{"docA", "docB"} {
		d := findDoc(result, id)
		if d == nil {
			t.Fatalf("结果中未找到 %s", id)
		}
		if !floatEq(d.Score, float32(expect)) {
			t.Errorf("%s 分数不符：期望 %f，实际 %f", id, expect, d.Score)
		}
	}
}
