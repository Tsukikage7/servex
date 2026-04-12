// Package extractor 提供基于 LLM 的信息提取功能，支持实体识别、关系抽取、关键词提取和文本摘要.
package extractor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Tsukikage7/servex/llm"
)

// 提取器错误类型.
var (
	// ErrNilModel 模型为 nil 时返回.
	ErrNilModel = errors.New("extractor: model is nil")
	// ErrEmptyText 文本为空时返回.
	ErrEmptyText = errors.New("extractor: empty text")
)

// Entity 提取的实体.
type Entity struct {
	// Text 原文中的文本.
	Text string `json:"text"`
	// Type 实体类型（如 person、organization、location）.
	Type string `json:"type"`
	// Start 起始位置，-1 表示未知.
	Start int `json:"start"`
	// End 结束位置.
	End int `json:"end"`
	// Metadata 附加元数据.
	Metadata map[string]any `json:"metadata,omitzero"`
}

// Relation 实体关系.
type Relation struct {
	// Subject 主语实体.
	Subject string `json:"subject"`
	// Predicate 谓语关系.
	Predicate string `json:"predicate"`
	// Object 宾语实体.
	Object string `json:"object"`
}

// Keyword 关键词.
type Keyword struct {
	// Word 关键词文本.
	Word string `json:"word"`
	// Score 重要度分数（0.0-1.0）.
	Score float64 `json:"score"`
}

// Summary 摘要.
type Summary struct {
	// Text 摘要文本.
	Text string `json:"text"`
	// Sentences 摘要句子数.
	Sentences int `json:"sentences"`
}

// Result 提取结果.
type Result struct {
	// Entities 实体列表.
	Entities []Entity `json:"entities,omitzero"`
	// Relations 关系列表.
	Relations []Relation `json:"relations,omitzero"`
	// Keywords 关键词列表.
	Keywords []Keyword `json:"keywords,omitzero"`
	// Summary 摘要.
	Summary *Summary `json:"summary,omitzero"`
}

// Extractor 信息提取器接口.
type Extractor interface {
	// Extract 从文本中提取信息.
	Extract(ctx context.Context, text string) (*Result, error)
}

// options 提取器内部选项.
type options struct {
	// callOpts 底层模型调用选项.
	callOpts []llm.CallOption
}

// Option 提取器选项函数.
type Option func(*options)

// WithCallOptions 设置底层模型调用选项.
func WithCallOptions(opts ...llm.CallOption) Option {
	return func(o *options) {
		o.callOpts = append(o.callOpts, opts...)
	}
}

// ── 实体识别 ─────────────────────────────────────────────────────────────────

// entityExtractor 实体识别提取器.
type entityExtractor struct {
	model       llm.ChatModel
	opts        *options
	entityTypes []string
}

// NewEntityExtractor 创建实体识别提取器.
// entityTypes 为要提取的实体类型列表，如 "person"、"organization"、"location"、"date".
func NewEntityExtractor(model llm.ChatModel, entityTypes []string, opts ...Option) Extractor {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return &entityExtractor{
		model:       model,
		opts:        o,
		entityTypes: entityTypes,
	}
}

// Extract 从文本中识别实体.
func (e *entityExtractor) Extract(ctx context.Context, text string) (*Result, error) {
	if e.model == nil {
		return nil, ErrNilModel
	}
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmptyText
	}

	typeList := strings.Join(e.entityTypes, "、")
	sysPrompt := fmt.Sprintf(
		"从文本中识别以下类型的实体：%s。\n"+
			"输出 JSON 数组，每个元素包含：text（原文）、type（实体类型）、start（起始位置，不确定填 -1）、end（结束位置）、metadata（附加信息，可为空对象）。\n"+
			"仅输出 JSON 数组，不要其他内容。示例：[{\"text\":\"张三\",\"type\":\"person\",\"start\":0,\"end\":2,\"metadata\":{}}]",
		typeList,
	)

	messages := []llm.Message{
		llm.SystemMessage(sysPrompt),
		llm.UserMessage(text),
	}

	resp, err := e.model.Generate(ctx, messages, e.opts.callOpts...)
	if err != nil {
		return nil, fmt.Errorf("extractor: 模型调用失败: %w", err)
	}

	content := llm.ExtractJSON(resp.Message.Content)
	var entities []Entity
	if err := json.Unmarshal([]byte(content), &entities); err != nil {
		return nil, fmt.Errorf("extractor: JSON 解析失败: %w", err)
	}

	return &Result{Entities: entities}, nil
}

// ── 关系抽取 ─────────────────────────────────────────────────────────────────

// relationExtractor 关系抽取提取器.
type relationExtractor struct {
	model llm.ChatModel
	opts  *options
}

// NewRelationExtractor 创建关系抽取提取器.
func NewRelationExtractor(model llm.ChatModel, opts ...Option) Extractor {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return &relationExtractor{model: model, opts: o}
}

// Extract 从文本中抽取实体关系.
func (e *relationExtractor) Extract(ctx context.Context, text string) (*Result, error) {
	if e.model == nil {
		return nil, ErrNilModel
	}
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmptyText
	}

	sysPrompt := "从文本中抽取实体间的关系。\n" +
		"输出 JSON 数组，每个元素包含：subject（主语）、predicate（关系谓词）、object（宾语）。\n" +
		"仅输出 JSON 数组。示例：[{\"subject\":\"张三\",\"predicate\":\"供职于\",\"object\":\"字节跳动\"}]"

	messages := []llm.Message{
		llm.SystemMessage(sysPrompt),
		llm.UserMessage(text),
	}

	resp, err := e.model.Generate(ctx, messages, e.opts.callOpts...)
	if err != nil {
		return nil, fmt.Errorf("extractor: 模型调用失败: %w", err)
	}

	content := llm.ExtractJSON(resp.Message.Content)
	var relations []Relation
	if err := json.Unmarshal([]byte(content), &relations); err != nil {
		return nil, fmt.Errorf("extractor: JSON 解析失败: %w", err)
	}

	return &Result{Relations: relations}, nil
}

// ── 关键词提取 ───────────────────────────────────────────────────────────────

// keywordOptions 关键词提取选项.
type keywordOptions struct {
	// callOpts 底层模型调用选项.
	callOpts []llm.CallOption
	// maxKeywords 最大关键词数量，默认 10.
	maxKeywords int
}

// KeywordOption 关键词提取选项函数.
type KeywordOption func(*keywordOptions)

// WithMaxKeywords 设置最大关键词数量（默认 10）.
func WithMaxKeywords(n int) KeywordOption {
	return func(o *keywordOptions) {
		o.maxKeywords = n
	}
}

// keywordExtractor 关键词提取器.
type keywordExtractor struct {
	model llm.ChatModel
	opts  *keywordOptions
}

// NewKeywordExtractor 创建关键词提取器.
func NewKeywordExtractor(model llm.ChatModel, opts ...KeywordOption) Extractor {
	o := &keywordOptions{maxKeywords: 10}
	for _, opt := range opts {
		opt(o)
	}
	return &keywordExtractor{model: model, opts: o}
}

// Extract 从文本中提取关键词.
func (e *keywordExtractor) Extract(ctx context.Context, text string) (*Result, error) {
	if e.model == nil {
		return nil, ErrNilModel
	}
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmptyText
	}

	sysPrompt := fmt.Sprintf(
		"从文本中提取最多 %d 个关键词，按重要度排序。\n"+
			"输出 JSON 数组，每个元素包含：word（关键词）、score（重要度 0-1）。\n"+
			"仅输出 JSON 数组，按 score 降序。示例：[{\"word\":\"人工智能\",\"score\":0.95},{\"word\":\"机器学习\",\"score\":0.8}]",
		e.opts.maxKeywords,
	)

	messages := []llm.Message{
		llm.SystemMessage(sysPrompt),
		llm.UserMessage(text),
	}

	resp, err := e.model.Generate(ctx, messages, e.opts.callOpts...)
	if err != nil {
		return nil, fmt.Errorf("extractor: 模型调用失败: %w", err)
	}

	content := llm.ExtractJSON(resp.Message.Content)
	var keywords []Keyword
	if err := json.Unmarshal([]byte(content), &keywords); err != nil {
		return nil, fmt.Errorf("extractor: JSON 解析失败: %w", err)
	}

	// 截断至最大数量.
	if len(keywords) > e.opts.maxKeywords {
		keywords = keywords[:e.opts.maxKeywords]
	}

	return &Result{Keywords: keywords}, nil
}

// ── 文本摘要 ─────────────────────────────────────────────────────────────────

// summaryOptions 摘要选项.
type summaryOptions struct {
	// callOpts 底层模型调用选项.
	callOpts []llm.CallOption
	// maxSentences 摘要最大句子数，默认 3.
	maxSentences int
	// language 输出语言，为空则与输入语言相同.
	language string
}

// SummaryOption 摘要选项函数.
type SummaryOption func(*summaryOptions)

// WithMaxSentences 设置摘要最大句子数（默认 3）.
func WithMaxSentences(n int) SummaryOption {
	return func(o *summaryOptions) {
		o.maxSentences = n
	}
}

// WithLanguage 设置摘要输出语言（如 "zh"、"en"）.
func WithLanguage(lang string) SummaryOption {
	return func(o *summaryOptions) {
		o.language = lang
	}
}

// summarizer 文本摘要提取器.
type summarizer struct {
	model llm.ChatModel
	opts  *summaryOptions
}

// NewSummarizer 创建文本摘要提取器.
func NewSummarizer(model llm.ChatModel, opts ...SummaryOption) Extractor {
	o := &summaryOptions{maxSentences: 3}
	for _, opt := range opts {
		opt(o)
	}
	return &summarizer{model: model, opts: o}
}

// Extract 对文本生成摘要.
func (s *summarizer) Extract(ctx context.Context, text string) (*Result, error) {
	if s.model == nil {
		return nil, ErrNilModel
	}
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmptyText
	}

	var langHint string
	if s.opts.language != "" {
		langHint = fmt.Sprintf("，摘要语言为 %s", s.opts.language)
	}

	sysPrompt := fmt.Sprintf(
		"对文本生成简洁摘要，不超过 %d 句话%s。\n"+
			"输出 JSON 对象，包含：text（摘要文本）、sentences（实际句子数）。\n"+
			"仅输出 JSON 对象。示例：{\"text\":\"这是摘要内容。\",\"sentences\":1}",
		s.opts.maxSentences,
		langHint,
	)

	messages := []llm.Message{
		llm.SystemMessage(sysPrompt),
		llm.UserMessage(text),
	}

	resp, err := s.model.Generate(ctx, messages, s.opts.callOpts...)
	if err != nil {
		return nil, fmt.Errorf("extractor: 模型调用失败: %w", err)
	}

	content := llm.ExtractJSON(resp.Message.Content)
	var summary Summary
	if err := json.Unmarshal([]byte(content), &summary); err != nil {
		return nil, fmt.Errorf("extractor: JSON 解析失败: %w", err)
	}

	return &Result{Summary: &summary}, nil
}
