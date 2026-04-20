package hybrid_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm/retrieval/hybrid"
	"github.com/Tsukikage7/servex/v2/llm/retrieval/rag"
)

// mockVectorRetriever 演示用：按预设顺序返回向量召回结果.
type mockVectorRetriever struct {
	docs []rag.RetrievedDoc
}

// Retrieve 按 topK 截取预设结果返回.
func (m *mockVectorRetriever) Retrieve(_ context.Context, _ string, topK int) ([]rag.RetrievedDoc, error) {
	if topK > 0 && len(m.docs) > topK {
		return m.docs[:topK], nil
	}
	return m.docs, nil
}

// ExampleNewBM25Retriever 展示单独使用 BM25 召回器.
func ExampleNewBM25Retriever() {
	docs := []rag.Document{
		{ID: "d1", Content: "退款政策说明"},
		{ID: "d2", Content: "发货流程介绍"},
	}
	r := hybrid.NewBM25Retriever(docs)
	result, _ := r.Retrieve(context.Background(), "退款", 5)
	if len(result) > 0 {
		fmt.Println(result[0].ID)
	}
	// Output:
	// d1
}

// ExampleNew 展示 BM25 + mock 向量召回组合为 HybridRetriever.
func ExampleNew() {
	corpus := []rag.Document{
		{ID: "d1", Content: "关于退款的详细说明"},
		{ID: "d2", Content: "订单发货与物流查询"},
		{ID: "d3", Content: "退款流程与政策解读"},
	}

	bm25 := hybrid.NewBM25Retriever(corpus)
	vector := &mockVectorRetriever{docs: []rag.RetrievedDoc{
		{Document: rag.Document{ID: "d3", Content: "退款流程与政策解读"}, Score: 0.92},
		{Document: rag.Document{ID: "d1", Content: "关于退款的详细说明"}, Score: 0.85},
	}}

	h := hybrid.New(vector, bm25,
		hybrid.WithRRFK(60),
		hybrid.WithWeights(1.0, 1.0),
	)

	result, _ := h.Retrieve(context.Background(), "退款政策", 3)
	fmt.Println(result[0].ID)
	// Output:
	// d3
}
