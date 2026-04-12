// Package classifier 提供基于 LLM 的文本分类功能，支持意图识别、情感分析、主题分类等.
package classifier

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Tsukikage7/servex/llm"
)

// 分类器错误类型.
var (
	// ErrNilModel 模型为 nil 时返回.
	ErrNilModel = errors.New("classifier: model is nil")
	// ErrEmptyText 文本为空时返回.
	ErrEmptyText = errors.New("classifier: empty text")
	// ErrNoLabels 未定义任何标签时返回.
	ErrNoLabels = errors.New("classifier: no labels defined")
)

// Label 分类标签.
type Label struct {
	// Name 标签名称.
	Name string `json:"name"`
	// Score 置信度分数，范围 0.0-1.0.
	Score float64 `json:"score"`
	// Description 标签描述或原因（可选）.
	Description string `json:"description,omitempty"`
}

// Result 分类结果.
type Result struct {
	// Labels 所有标签列表，按 Score 降序排列.
	Labels []Label `json:"labels"`
	// Best 最高分标签.
	Best Label `json:"best"`
}

// Classifier 分类器接口.
type Classifier interface {
	// Classify 对单段文本进行分类.
	Classify(ctx context.Context, text string) (*Result, error)
	// ClassifyMessages 对消息列表进行分类（拼接所有内容后分类）.
	ClassifyMessages(ctx context.Context, messages []llm.Message) (*Result, error)
}

// options 分类器内部选项.
type options struct {
	// callOpts 底层模型调用选项.
	callOpts []llm.CallOption
	// topN 返回前 N 个标签，0 表示全部返回.
	topN int
}

// Option 分类器选项函数.
type Option func(*options)

// WithCallOptions 设置底层模型调用选项.
func WithCallOptions(opts ...llm.CallOption) Option {
	return func(o *options) {
		o.callOpts = append(o.callOpts, opts...)
	}
}

// WithTopN 设置返回前 N 个标签（默认返回全部）.
func WithTopN(n int) Option {
	return func(o *options) {
		o.topN = n
	}
}

// llmClassifier 基于 LLM 的通用分类器.
type llmClassifier struct {
	// model 底层聊天模型.
	model llm.ChatModel
	// opts 分类器选项.
	opts *options
	// systemPrompt 系统提示.
	systemPrompt string
}

// Classify 使用 LLM 对文本进行分类.
func (c *llmClassifier) Classify(ctx context.Context, text string) (*Result, error) {
	if c.model == nil {
		return nil, ErrNilModel
	}
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmptyText
	}

	messages := []llm.Message{
		llm.SystemMessage(c.systemPrompt),
		llm.UserMessage(text),
	}

	resp, err := c.model.Generate(ctx, messages, c.opts.callOpts...)
	if err != nil {
		return nil, fmt.Errorf("classifier: 模型调用失败: %w", err)
	}

	return c.parseResponse(resp.Message.Content)
}

// ClassifyMessages 将消息列表的所有内容拼接后进行分类.
func (c *llmClassifier) ClassifyMessages(ctx context.Context, messages []llm.Message) (*Result, error) {
	parts := make([]string, 0, len(messages))
	for _, msg := range messages {
		if msg.Content != "" {
			parts = append(parts, msg.Content)
		}
		for _, part := range msg.Parts {
			if part.Type == llm.ContentTypeText && part.Text != "" {
				parts = append(parts, part.Text)
			}
		}
	}
	return c.Classify(ctx, strings.Join(parts, "\n"))
}

// parseResponse 解析 LLM 返回的 JSON 数组，生成分类结果.
func (c *llmClassifier) parseResponse(content string) (*Result, error) {
	content = llm.ExtractJSON(content)

	var labels []Label
	if err := json.Unmarshal([]byte(content), &labels); err != nil {
		return nil, fmt.Errorf("classifier: JSON 解析失败: %w", err)
	}

	// 按分数降序排列.
	slices.SortFunc(labels, func(a, b Label) int {
		return cmp.Compare(b.Score, a.Score)
	})

	// 截取前 N 个.
	if c.opts.topN > 0 && len(labels) > c.opts.topN {
		labels = labels[:c.opts.topN]
	}

	result := &Result{Labels: labels}
	if len(labels) > 0 {
		result.Best = labels[0]
	}
	return result, nil
}

// newClassifier 创建通用分类器实例.
func newClassifier(model llm.ChatModel, systemPrompt string, opts []Option) *llmClassifier {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return &llmClassifier{
		model:        model,
		opts:         o,
		systemPrompt: systemPrompt,
	}
}

// NewIntentClassifier 创建意图识别分类器.
// intents 为意图名称到描述的映射.
func NewIntentClassifier(model llm.ChatModel, intents map[string]string, opts ...Option) Classifier {
	if len(intents) == 0 {
		return &errClassifier{err: ErrNoLabels}
	}
	parts := make([]string, 0, len(intents))
	for name, desc := range intents {
		parts = append(parts, fmt.Sprintf("- %s: %s", name, desc))
	}
	sysPrompt := fmt.Sprintf(
		"判断用户意图。可选意图：\n%s\n\n输出 JSON 数组，每个元素包含 name（意图名）、score（置信度 0-1）、description（判断理由）。"+
			"仅输出 JSON，不要任何额外内容。示例：[{\"name\": \"greeting\", \"score\": 0.95, \"description\": \"用户在打招呼\"}]",
		strings.Join(parts, "\n"),
	)
	return newClassifier(model, sysPrompt, opts)
}

// NewSentimentClassifier 创建情感分析分类器（正面/中性/负面）.
func NewSentimentClassifier(model llm.ChatModel, opts ...Option) Classifier {
	sysPrompt := "分析文本情感，使用以下标签：positive（正面）、neutral（中性）、negative（负面）。" +
		"输出 JSON 数组，每个元素包含 name（标签）、score（置信度 0-1）、description（理由）。" +
		"仅输出 JSON。示例：[{\"name\": \"positive\", \"score\": 0.9, \"description\": \"文本表达积极情绪\"}, " +
		"{\"name\": \"neutral\", \"score\": 0.08, \"description\": \"\"}, {\"name\": \"negative\", \"score\": 0.02, \"description\": \"\"}]"
	return newClassifier(model, sysPrompt, opts)
}

// NewTopicClassifier 创建主题分类器.
// topics 为可选主题列表，为空则由 LLM 自行判断主题.
func NewTopicClassifier(model llm.ChatModel, topics []string, opts ...Option) Classifier {
	var sysPrompt string
	if len(topics) > 0 {
		sysPrompt = fmt.Sprintf(
			"从以下主题中选择最匹配的：%s。"+
				"输出 JSON 数组，每个元素包含 name（主题名）、score（置信度 0-1）、description（理由）。仅输出 JSON。",
			strings.Join(topics, "、"),
		)
	} else {
		sysPrompt = "自动识别文本主题，可输出多个相关主题。" +
			"输出 JSON 数组，每个元素包含 name（主题名）、score（置信度 0-1）、description（理由）。仅输出 JSON。"
	}
	return newClassifier(model, sysPrompt, opts)
}

// NewLanguageClassifier 创建语言检测分类器.
func NewLanguageClassifier(model llm.ChatModel, opts ...Option) Classifier {
	sysPrompt := "识别文本语言，输出语言代码（zh/en/ja/ko/fr/de/es/...）。" +
		"输出 JSON 数组，每个元素包含 name（语言代码）、score（置信度 0-1）、description（语言全称）。仅输出 JSON。" +
		"示例：[{\"name\": \"zh\", \"score\": 0.98, \"description\": \"简体中文\"}]"
	return newClassifier(model, sysPrompt, opts)
}

// NewToxicityClassifier 创建毒性检测分类器.
func NewToxicityClassifier(model llm.ChatModel, opts ...Option) Classifier {
	sysPrompt := "评估文本毒性，使用以下类别：toxic（有毒/有害）、safe（安全/无害）。" +
		"输出 JSON 数组，每个元素包含 name（类别）、score（置信度 0-1）、description（理由）。仅输出 JSON。"
	return newClassifier(model, sysPrompt, opts)
}

// NewRouterClassifier 创建 Agent/工具路由分类器，根据输入选择最匹配的路由.
// routes 为路由名称到描述的映射.
func NewRouterClassifier(model llm.ChatModel, routes map[string]string, opts ...Option) Classifier {
	if len(routes) == 0 {
		return &errClassifier{err: ErrNoLabels}
	}
	parts := make([]string, 0, len(routes))
	for name, desc := range routes {
		parts = append(parts, fmt.Sprintf("- %s: %s", name, desc))
	}
	sysPrompt := fmt.Sprintf(
		"根据输入内容选择最匹配的路由。可选路由：\n%s\n\n"+
			"输出 JSON 数组，每个元素包含 name（路由名）、score（匹配度 0-1）、description（选择理由）。仅输出 JSON。",
		strings.Join(parts, "\n"),
	)
	return newClassifier(model, sysPrompt, opts)
}

// NewCustomClassifier 创建自定义分类器.
// labels 为标签列表，systemPrompt 为自定义系统提示（应说明如何输出 JSON 数组）.
func NewCustomClassifier(model llm.ChatModel, labels []string, systemPrompt string, opts ...Option) Classifier {
	if len(labels) == 0 {
		return &errClassifier{err: ErrNoLabels}
	}
	prompt := fmt.Sprintf(
		"%s\n可用标签：%s\n输出 JSON 数组，每个元素包含 name、score（0-1）、description。仅输出 JSON。",
		systemPrompt,
		strings.Join(labels, "、"),
	)
	return newClassifier(model, prompt, opts)
}

// errClassifier 始终返回固定错误的分类器，用于构造参数校验失败的情况.
type errClassifier struct {
	err error
}

// Classify 始终返回构造时的错误.
func (e *errClassifier) Classify(_ context.Context, _ string) (*Result, error) {
	return nil, e.err
}

// ClassifyMessages 始终返回构造时的错误.
func (e *errClassifier) ClassifyMessages(_ context.Context, _ []llm.Message) (*Result, error) {
	return nil, e.err
}
