// Package memory 提供基于内存的 VectorStore 实现，用于测试和原型开发.
package memory

import (
	"context"
	"math"
	"sync"

	"github.com/Tsukikage7/servex/v2/llm/retrieval/vectorstore"
)

// Store 内存向量存储.
type Store struct {
	mu   sync.RWMutex
	docs []vectorstore.Document
}

// New 创建内存向量存储实例.
func New() *Store {
	return &Store{}
}

// AddDocuments 批量添加文档.
func (s *Store) AddDocuments(_ context.Context, docs []vectorstore.Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.docs = append(s.docs, docs...)
	return nil
}

// SimilaritySearch 基于余弦相似度搜索最相似的 k 条文档.
func (s *Store) SimilaritySearch(_ context.Context, query []float32, k int, opts ...vectorstore.SearchOption) ([]vectorstore.SearchResult, error) {
	o := vectorstore.ApplySearchOptions(opts)

	s.mu.RLock()
	defer s.mu.RUnlock()

	type scored struct {
		doc   vectorstore.Document
		score float32
	}

	var candidates []scored
	for _, doc := range s.docs {
		if o.Filter() != nil && !matchFilter(doc.Metadata, o.Filter()) {
			continue
		}
		score := cosineSimilarity(query, doc.Vector)
		if o.ScoreThreshold() != nil && score < *o.ScoreThreshold() {
			continue
		}
		candidates = append(candidates, scored{doc: doc, score: score})
	}

	// 按分数降序插入排序
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0 && candidates[j].score > candidates[j-1].score; j-- {
			candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
		}
	}

	if k > len(candidates) {
		k = len(candidates)
	}
	results := make([]vectorstore.SearchResult, k)
	for i := range k {
		results[i] = vectorstore.SearchResult{
			Document: candidates[i].doc,
			Score:    candidates[i].score,
		}
	}
	return results, nil
}

// Delete 根据 ID 删除文档.
func (s *Store) Delete(_ context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}

	filtered := s.docs[:0]
	for _, doc := range s.docs {
		if _, del := idSet[doc.ID]; !del {
			filtered = append(filtered, doc)
		}
	}
	s.docs = filtered
	return nil
}

// cosineSimilarity 计算两个向量的余弦相似度.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return float32(dot / denom)
}

// matchFilter 检查文档元数据是否满足所有过滤条件（精确匹配）.
func matchFilter(meta, filter map[string]any) bool {
	for k, v := range filter {
		if meta[k] != v {
			return false
		}
	}
	return true
}

// 编译期接口断言.
var _ vectorstore.VectorStore = (*Store)(nil)
