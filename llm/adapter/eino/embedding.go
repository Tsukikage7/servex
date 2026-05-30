package eino

import (
	"context"

	"github.com/cloudwego/eino/components/embedding"

	"github.com/Tsukikage7/servex/v2/llm"
)

// EmbeddingModel 将 Eino Embedder 适配为 servex llm.EmbeddingModel.
type EmbeddingModel struct {
	embedder embedding.Embedder
}

// NewEmbeddingModel 创建 Eino 到 servex 的 EmbeddingModel 适配器.
func NewEmbeddingModel(embedder embedding.Embedder) (*EmbeddingModel, error) {
	if embedder == nil {
		return nil, ErrNilEmbedder
	}
	return &EmbeddingModel{embedder: embedder}, nil
}

// Base 返回底层 Eino embedding 组件.
func (m *EmbeddingModel) Base() embedding.Embedder {
	return m.embedder
}

// EmbedTexts 执行文本向量化.
func (m *EmbeddingModel) EmbedTexts(ctx context.Context, texts []string, opts ...llm.CallOption) (*llm.EmbedResponse, error) {
	vecs, err := m.embedder.EmbedStrings(ctx, texts, toEinoEmbeddingOptions(opts)...)
	if err != nil {
		return nil, err
	}
	return &llm.EmbedResponse{
		Embeddings: float64ToFloat32(vecs),
		ModelID:    llm.ApplyOptions(opts).Model,
	}, nil
}

var _ llm.EmbeddingModel = (*EmbeddingModel)(nil)

// AsEmbedder 将 servex EmbeddingModel 适配为 Eino Embedder.
func AsEmbedder(model llm.EmbeddingModel) (embedding.Embedder, error) {
	if model == nil {
		return nil, ErrNilEmbedder
	}
	return &servexEmbedder{model: model}, nil
}

type servexEmbedder struct {
	model llm.EmbeddingModel
}

func (e *servexEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	resp, err := e.model.EmbedTexts(ctx, texts, toServexEmbeddingOptions(opts)...)
	if err != nil {
		return nil, err
	}
	return float32ToFloat64(resp.Embeddings), nil
}

func toEinoEmbeddingOptions(opts []llm.CallOption) []embedding.Option {
	applied := llm.ApplyOptions(opts)
	if applied.Model == "" {
		return nil
	}
	return []embedding.Option{embedding.WithModel(applied.Model)}
}

func toServexEmbeddingOptions(opts []embedding.Option) []llm.CallOption {
	common := embedding.GetCommonOptions(nil, opts...)
	if common.Model == nil {
		return nil
	}
	return []llm.CallOption{llm.WithModel(*common.Model)}
}

func float64ToFloat32(vecs [][]float64) [][]float32 {
	out := make([][]float32, 0, len(vecs))
	for _, vec := range vecs {
		converted := make([]float32, 0, len(vec))
		for _, v := range vec {
			converted = append(converted, float32(v))
		}
		out = append(out, converted)
	}
	return out
}

func float32ToFloat64(vecs [][]float32) [][]float64 {
	out := make([][]float64, 0, len(vecs))
	for _, vec := range vecs {
		converted := make([]float64, 0, len(vec))
		for _, v := range vec {
			converted = append(converted, float64(v))
		}
		out = append(out, converted)
	}
	return out
}
