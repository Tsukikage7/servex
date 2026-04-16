// Package vectorstore 提供向量存储的统一接口抽象.
package vectorstore

import "context"

// Document 向量存储中的文档.
type Document struct {
	// ID 文档唯一标识符.
	ID string
	// Content 文档文本内容.
	Content string
	// Vector 文档的向量表示（嵌入向量）.
	Vector []float32
	// Metadata 文档元数据（可用于过滤）.
	Metadata map[string]any
}

// SearchResult 相似度搜索结果.
type SearchResult struct {
	// Document 匹配的文档.
	Document Document
	// Score 相似度分数（范围视具体实现而定，通常越高越相似）.
	Score float32
}

// VectorStore 向量存储接口.
type VectorStore interface {
	// AddDocuments 批量添加文档（含向量）到存储.
	AddDocuments(ctx context.Context, docs []Document) error
	// SimilaritySearch 基于查询向量搜索最相似的 k 条文档.
	SimilaritySearch(ctx context.Context, query []float32, k int, opts ...SearchOption) ([]SearchResult, error)
	// Delete 根据 ID 列表删除文档.
	Delete(ctx context.Context, ids []string) error
}

// SearchOption 搜索选项.
type SearchOption func(*searchOptions)

// searchOptions 内部搜索选项.
type searchOptions struct {
	filter         map[string]any
	scoreThreshold *float32
	textQuery      string
}

// ApplySearchOptions 应用搜索选项.
func ApplySearchOptions(opts []SearchOption) searchOptions {
	var o searchOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// WithFilter 设置元数据过滤条件（仅返回满足条件的文档）.
func WithFilter(filter map[string]any) SearchOption {
	return func(o *searchOptions) { o.filter = filter }
}

// WithScoreThreshold 设置相似度分数阈值（仅返回分数高于阈值的结果）.
func WithScoreThreshold(threshold float32) SearchOption {
	return func(o *searchOptions) { o.scoreThreshold = &threshold }
}

// WithTextQuery 设置 BM25 文本查询，与向量搜索混合使用.
func WithTextQuery(text string) SearchOption {
	return func(o *searchOptions) { o.textQuery = text }
}

// Filter 返回元数据过滤条件.
func (o searchOptions) Filter() map[string]any { return o.filter }

// ScoreThreshold 返回分数阈值指针，未设置时返回 nil.
func (o searchOptions) ScoreThreshold() *float32 { return o.scoreThreshold }

// TextQuery 返回 BM25 文本查询字符串，未设置时返回空字符串.
func (o searchOptions) TextQuery() string { return o.textQuery }
