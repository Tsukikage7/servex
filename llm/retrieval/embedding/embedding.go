// Package embedding 提供嵌入向量辅助工具函数.
package embedding

import (
	"context"
	"fmt"
	"math"

	"github.com/Tsukikage7/servex/v2/llm"
)

// BatchEmbed 批量嵌入文本，将长文本列表按 batchSize 分批调用 EmbeddingModel.
// 适合文本数量超过 Provider 单次调用限制的场景.
func BatchEmbed(ctx context.Context, model llm.EmbeddingModel, texts []string, batchSize int, opts ...llm.CallOption) (*llm.EmbedResponse, error) {
	if len(texts) == 0 {
		return &llm.EmbedResponse{}, nil
	}
	if batchSize <= 0 {
		batchSize = 100
	}

	result := &llm.EmbedResponse{
		Embeddings: make([][]float32, 0, len(texts)),
	}

	for start := 0; start < len(texts); start += batchSize {
		end := start + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[start:end]

		resp, err := model.EmbedTexts(ctx, batch, opts...)
		if err != nil {
			return nil, fmt.Errorf("embedding: 第 %d 批嵌入失败: %w", start/batchSize+1, err)
		}

		result.Embeddings = append(result.Embeddings, resp.Embeddings...)
		result.Usage.Add(resp.Usage)
		if result.ModelID == "" {
			result.ModelID = resp.ModelID
		}
	}

	return result, nil
}

// CosineSimilarity 计算两个向量的余弦相似度.
// 返回值范围 [-1, 1]，1 表示完全相同方向，-1 表示完全相反，0 表示正交.
// 若任一向量为零向量，返回 0.
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return float32(dotProduct / denom)
}
