// Package rerank 提供 RAG 检索结果重排序功能，用于提升检索质量.
//
// 支持基于 LLM 的重排序、基于 Embedding 余弦相似度的重排序以及基于外部 Cross-Encoder API 的重排序.
package rerank

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/Tsukikage7/servex/llm"
	"github.com/Tsukikage7/servex/llm/retrieval/embedding"
	"github.com/Tsukikage7/servex/llm/retrieval/rag"
)

// 预定义错误.
var (
	// ErrNilModel 未提供模型.
	ErrNilModel = errors.New("rerank: model is nil")
	// ErrEmptyDocs 待重排序文档列表为空.
	ErrEmptyDocs = errors.New("rerank: no documents to rerank")
	// ErrEmptyEndpoint 未提供外部 API 端点.
	ErrEmptyEndpoint = errors.New("rerank: endpoint is empty")
	// ErrAPIFailed 外部 API 请求失败.
	ErrAPIFailed = errors.New("rerank: API request failed")
)

// Reranker 重排序接口.
type Reranker interface {
	// Rerank 对检索文档重新排序，返回按相关性降序排列的文档列表.
	Rerank(ctx context.Context, query string, docs []rag.RetrievedDoc) ([]rag.RetrievedDoc, error)
}

// Option 重排序选项函数.
type Option func(*options)

// options 通用重排序配置.
type options struct {
	// topN 返回的最大文档数，0 表示返回全部.
	topN int
	// batchSize LLM 重排序时每批处理的文档数.
	batchSize int
}

// WithTopN 设置最多返回的文档数，0 表示返回全部.
func WithTopN(n int) Option {
	return func(o *options) {
		o.topN = n
	}
}

// WithBatchSize 设置 LLM 重排序时每批处理的文档数.
func WithBatchSize(n int) Option {
	return func(o *options) {
		o.batchSize = n
	}
}

// applyTopN 对结果集截取前 topN 条，topN<=0 时返回全部.
func applyTopN(docs []rag.RetrievedDoc, topN int) []rag.RetrievedDoc {
	if topN > 0 && topN < len(docs) {
		return docs[:topN]
	}
	return docs
}

// ──────────────────────────────────────────
// LLM Reranker
// ──────────────────────────────────────────

// llmReranker 基于 LLM 评分的重排序实现.
type llmReranker struct {
	model llm.ChatModel
	opts  options
}

// NewLLMReranker 创建基于 LLM 的重排序器.
// model 为聊天模型，用于对文档打分；opts 为可选配置.
func NewLLMReranker(model llm.ChatModel, opts ...Option) Reranker {
	o := options{batchSize: 10}
	for _, opt := range opts {
		opt(&o)
	}
	return &llmReranker{model: model, opts: o}
}

// llmScoreItem LLM 返回的单条评分记录.
type llmScoreItem struct {
	Index int     `json:"index"`
	Score float32 `json:"score"`
}

// Rerank 使用 LLM 对文档按查询相关性评分后重排序.
func (r *llmReranker) Rerank(ctx context.Context, query string, docs []rag.RetrievedDoc) ([]rag.RetrievedDoc, error) {
	if r.model == nil {
		return nil, ErrNilModel
	}
	if len(docs) == 0 {
		return nil, ErrEmptyDocs
	}

	batchSize := r.opts.batchSize
	if batchSize <= 0 {
		batchSize = 10
	}

	// 存储每个文档的评分，按原始索引.
	scores := make([]float32, len(docs))

	// 分批处理文档.
	for start := 0; start < len(docs); start += batchSize {
		end := start + batchSize
		if end > len(docs) {
			end = len(docs)
		}
		batch := docs[start:end]

		// 构造提示词.
		prompt := buildLLMPrompt(query, batch, start)

		resp, err := r.model.Generate(ctx, []llm.Message{
			llm.UserMessage(prompt),
		})
		if err != nil {
			return nil, fmt.Errorf("rerank: LLM 调用失败: %w", err)
		}

		// 解析 JSON 评分数组.
		items, err := parseLLMScores(resp.Message.Content)
		if err != nil {
			return nil, fmt.Errorf("rerank: 解析 LLM 评分失败: %w", err)
		}

		// 将评分写回对应位置.
		for _, item := range items {
			globalIdx := item.Index
			if globalIdx >= 0 && globalIdx < len(docs) {
				scores[globalIdx] = item.Score
			}
		}
	}

	// 构造带评分的文档列表并排序.
	ranked := make([]rag.RetrievedDoc, len(docs))
	copy(ranked, docs)
	for i := range ranked {
		ranked[i].Score = scores[i]
	}
	slices.SortFunc(ranked, func(a, b rag.RetrievedDoc) int {
		return cmp.Compare(b.Score, a.Score)
	})

	return applyTopN(ranked, r.opts.topN), nil
}

// buildLLMPrompt 构造 LLM 评分提示词.
// offset 为本批文档在原始列表中的起始偏移，用于确保 index 全局唯一.
func buildLLMPrompt(query string, docs []rag.RetrievedDoc, offset int) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "给以下文档相对于查询的相关性打分(0-10)。查询:%s\n", query)
	for i, doc := range docs {
		fmt.Fprintf(&buf, "文档%d:%s\n", offset+i+1, doc.Content)
	}
	buf.WriteString("请输出JSON数组，格式为[{\"index\":全局文档索引(从0开始),\"score\":分数},...]，只输出JSON，不要其他文字。")
	return buf.String()
}

// parseLLMScores 从 LLM 响应文本中解析评分 JSON 数组.
// 支持响应中包含代码块标记的情况.
func parseLLMScores(content string) ([]llmScoreItem, error) {
	// 尝试从内容中提取 JSON 数组部分.
	start := -1
	end := -1
	for i, ch := range content {
		if ch == '[' && start == -1 {
			start = i
		}
		if ch == ']' {
			end = i + 1
		}
	}
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("未找到有效的 JSON 数组")
	}
	jsonStr := content[start:end]

	var items []llmScoreItem
	if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	return items, nil
}

// ──────────────────────────────────────────
// Embedding Reranker
// ──────────────────────────────────────────

// embeddingReranker 基于 Embedding 余弦相似度的重排序实现.
type embeddingReranker struct {
	model llm.EmbeddingModel
	opts  options
}

// NewEmbeddingReranker 创建基于 Embedding 的重排序器.
// model 为嵌入模型；opts 为可选配置.
func NewEmbeddingReranker(model llm.EmbeddingModel, opts ...Option) Reranker {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}
	return &embeddingReranker{model: model, opts: o}
}

// Rerank 使用 Embedding 余弦相似度对文档重排序.
func (r *embeddingReranker) Rerank(ctx context.Context, query string, docs []rag.RetrievedDoc) ([]rag.RetrievedDoc, error) {
	if r.model == nil {
		return nil, ErrNilModel
	}
	if len(docs) == 0 {
		return nil, ErrEmptyDocs
	}

	// 将 query 和所有文档内容合并为一批进行嵌入.
	texts := make([]string, 0, len(docs)+1)
	texts = append(texts, query)
	for _, doc := range docs {
		texts = append(texts, doc.Content)
	}

	embedResp, err := r.model.EmbedTexts(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("rerank: 嵌入失败: %w", err)
	}
	if len(embedResp.Embeddings) < len(docs)+1 {
		return nil, fmt.Errorf("rerank: 嵌入模型返回向量数量不足")
	}

	queryVec := embedResp.Embeddings[0]

	// 计算每个文档与 query 的余弦相似度.
	ranked := make([]rag.RetrievedDoc, len(docs))
	copy(ranked, docs)
	for i := range ranked {
		sim := embedding.CosineSimilarity(queryVec, embedResp.Embeddings[i+1])
		ranked[i].Score = sim
	}

	// 按相似度降序排序.
	slices.SortFunc(ranked, func(a, b rag.RetrievedDoc) int {
		return cmp.Compare(b.Score, a.Score)
	})

	return applyTopN(ranked, r.opts.topN), nil
}

// ──────────────────────────────────────────
// Cross-Encoder Reranker
// ──────────────────────────────────────────

// CrossEncoderOption Cross-Encoder 重排序器选项函数.
type CrossEncoderOption func(*crossEncoderOptions)

// crossEncoderOptions Cross-Encoder 重排序器配置.
type crossEncoderOptions struct {
	// apiKey 可选的 API 密钥，添加到请求头.
	apiKey string
	// model 可选的模型名称，添加到请求体.
	model string
	// topN 返回的最大文档数，0 表示返回全部.
	topN int
}

// WithAPIKey 设置 Cross-Encoder API 密钥.
func WithAPIKey(key string) CrossEncoderOption {
	return func(o *crossEncoderOptions) {
		o.apiKey = key
	}
}

// WithModel 设置 Cross-Encoder 模型名称.
func WithModel(model string) CrossEncoderOption {
	return func(o *crossEncoderOptions) {
		o.model = model
	}
}

// crossEncoderReranker 基于外部 Cross-Encoder API 的重排序实现.
type crossEncoderReranker struct {
	endpoint string
	opts     crossEncoderOptions
	client   *http.Client
}

// NewCrossEncoderReranker 创建基于外部 Cross-Encoder API 的重排序器.
// endpoint 为 API 端点 URL；opts 为可选配置.
func NewCrossEncoderReranker(endpoint string, opts ...CrossEncoderOption) Reranker {
	o := crossEncoderOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	return &crossEncoderReranker{
		endpoint: endpoint,
		opts:     o,
		client:   &http.Client{},
	}
}

// crossEncoderRequest Cross-Encoder API 请求体.
type crossEncoderRequest struct {
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n,omitempty"`
	Model     string   `json:"model,omitempty"`
}

// crossEncoderResultItem Cross-Encoder API 响应中的单条结果.
type crossEncoderResultItem struct {
	Index          int     `json:"index"`
	RelevanceScore float32 `json:"relevance_score"`
}

// crossEncoderResponse Cross-Encoder API 响应体.
type crossEncoderResponse struct {
	Results []crossEncoderResultItem `json:"results"`
}

// Rerank 调用外部 Cross-Encoder API 对文档重排序.
func (r *crossEncoderReranker) Rerank(ctx context.Context, query string, docs []rag.RetrievedDoc) ([]rag.RetrievedDoc, error) {
	if r.endpoint == "" {
		return nil, ErrEmptyEndpoint
	}
	if len(docs) == 0 {
		return nil, ErrEmptyDocs
	}

	// 提取文档文本列表.
	docTexts := make([]string, len(docs))
	for i, doc := range docs {
		docTexts[i] = doc.Content
	}

	// 构造请求体.
	reqBody := crossEncoderRequest{
		Query:     query,
		Documents: docTexts,
		TopN:      r.opts.topN,
		Model:     r.opts.model,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("rerank: 序列化请求失败: %w", err)
	}

	// 创建 HTTP 请求.
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("rerank: 创建请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if r.opts.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.opts.apiKey)
	}

	// 发送请求.
	httpResp, err := r.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAPIFailed, err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("%w: 状态码 %d, 响应: %s", ErrAPIFailed, httpResp.StatusCode, string(body))
	}

	// 解析响应.
	var apiResp crossEncoderResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("rerank: 解析 API 响应失败: %w", err)
	}

	// 按 relevance_score 降序排列结果.
	slices.SortFunc(apiResp.Results, func(a, b crossEncoderResultItem) int {
		return cmp.Compare(b.RelevanceScore, a.RelevanceScore)
	})

	// 按结果顺序重新组织文档列表.
	ranked := make([]rag.RetrievedDoc, 0, len(apiResp.Results))
	for _, result := range apiResp.Results {
		idx := result.Index
		if idx < 0 || idx >= len(docs) {
			continue
		}
		doc := docs[idx]
		doc.Score = result.RelevanceScore
		ranked = append(ranked, doc)
	}

	// 若 API 未返回全部文档，将剩余未评分文档追加到末尾.
	ranked = appendUnranked(ranked, docs, len(apiResp.Results))

	return applyTopN(ranked, r.opts.topN), nil
}

// appendUnranked 将原始文档列表中未出现在 ranked 中的文档追加到末尾.
func appendUnranked(ranked []rag.RetrievedDoc, original []rag.RetrievedDoc, apiResultCount int) []rag.RetrievedDoc {
	if apiResultCount >= len(original) {
		return ranked
	}
	// 标记已在 ranked 中出现的文档（通过 ID 匹配）.
	seen := make(map[string]struct{}, len(ranked))
	for _, doc := range ranked {
		seen[doc.ID] = struct{}{}
	}
	for _, doc := range original {
		if _, ok := seen[doc.ID]; !ok {
			ranked = append(ranked, doc)
		}
	}
	return ranked
}
