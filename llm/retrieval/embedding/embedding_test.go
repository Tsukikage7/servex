package embedding_test

import (
	"context"
	"math"
	"testing"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/retrieval/embedding"
)

// mockEmbeddingModel 模拟嵌入模型.
type mockEmbeddingModel struct {
	callCount int
}

func (m *mockEmbeddingModel) EmbedTexts(_ context.Context, texts []string, _ ...llm.CallOption) (*llm.EmbedResponse, error) {
	m.callCount++
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		embeddings[i] = []float32{float32(i) * 0.1, float32(i) * 0.2}
	}
	return &llm.EmbedResponse{
		Embeddings: embeddings,
		Usage:      llm.Usage{PromptTokens: len(texts), TotalTokens: len(texts)},
		ModelID:    "mock-model",
	}, nil
}

func TestBatchEmbed_SingleBatch(t *testing.T) {
	model := &mockEmbeddingModel{}
	texts := []string{"a", "b", "c"}

	resp, err := embedding.BatchEmbed(t.Context(), model, texts, 10)
	if err != nil {
		t.Fatalf("BatchEmbed 失败: %v", err)
	}

	if len(resp.Embeddings) != 3 {
		t.Errorf("期望 3 个嵌入向量，得到 %d", len(resp.Embeddings))
	}
	if model.callCount != 1 {
		t.Errorf("期望调用 1 次，得到 %d 次", model.callCount)
	}
}

func TestBatchEmbed_MultipleBatches(t *testing.T) {
	model := &mockEmbeddingModel{}
	texts := make([]string, 10)
	for i := range texts {
		texts[i] = "text"
	}

	resp, err := embedding.BatchEmbed(t.Context(), model, texts, 3)
	if err != nil {
		t.Fatalf("BatchEmbed 失败: %v", err)
	}

	if len(resp.Embeddings) != 10 {
		t.Errorf("期望 10 个嵌入向量，得到 %d", len(resp.Embeddings))
	}
	// 10 个文本，每批 3 个，需要 4 次调用（3+3+3+1）
	if model.callCount != 4 {
		t.Errorf("期望调用 4 次，得到 %d 次", model.callCount)
	}
}

func TestBatchEmbed_Empty(t *testing.T) {
	model := &mockEmbeddingModel{}
	resp, err := embedding.BatchEmbed(t.Context(), model, nil, 10)
	if err != nil {
		t.Fatalf("空输入 BatchEmbed 失败: %v", err)
	}
	if len(resp.Embeddings) != 0 {
		t.Errorf("空输入期望 0 个嵌入向量，得到 %d", len(resp.Embeddings))
	}
	if model.callCount != 0 {
		t.Error("空输入不应调用模型")
	}
}

func TestCosineSimilarity_Identical(t *testing.T) {
	v := []float32{1, 0, 0}
	sim := embedding.CosineSimilarity(v, v)
	if math.Abs(float64(sim)-1.0) > 1e-6 {
		t.Errorf("相同向量相似度应为 1.0，得到 %f", sim)
	}
}

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{0, 1}
	sim := embedding.CosineSimilarity(a, b)
	if math.Abs(float64(sim)) > 1e-6 {
		t.Errorf("正交向量相似度应为 0，得到 %f", sim)
	}
}

func TestCosineSimilarity_Opposite(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{-1, 0}
	sim := embedding.CosineSimilarity(a, b)
	if math.Abs(float64(sim)+1.0) > 1e-6 {
		t.Errorf("反向向量相似度应为 -1.0，得到 %f", sim)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float32{0, 0}
	b := []float32{1, 0}
	sim := embedding.CosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("零向量相似度应为 0，得到 %f", sim)
	}
}
