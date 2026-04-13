// Package moderation 提供内容审核功能，支持按类别分类检测有害内容.
package moderation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Tsukikage7/servex/v2/llm"
)

// Category 审核类别.
type Category string

const (
	// CategoryViolence 暴力内容.
	CategoryViolence Category = "violence"
	// CategorySexual 色情内容.
	CategorySexual Category = "sexual"
	// CategoryHate 仇恨言论.
	CategoryHate Category = "hate"
	// CategorySelfHarm 自我伤害内容.
	CategorySelfHarm Category = "self_harm"
	// CategoryDangerous 危险内容.
	CategoryDangerous Category = "dangerous"
	// CategoryPolitical 政治敏感内容.
	CategoryPolitical Category = "political"
	// CategorySpam 垃圾信息.
	CategorySpam Category = "spam"
)

// AllCategories 所有内置类别.
var AllCategories = []Category{
	CategoryViolence,
	CategorySexual,
	CategoryHate,
	CategorySelfHarm,
	CategoryDangerous,
	CategoryPolitical,
	CategorySpam,
}

// Result 审核结果.
type Result struct {
	// Flagged 是否被标记为违规.
	Flagged bool `json:"flagged"`
	// Categories 各类别是否命中.
	Categories map[Category]bool `json:"categories"`
	// Scores 各类别置信度分数（0.0～1.0）.
	Scores map[Category]float64 `json:"scores"`
	// Reason 审核理由说明.
	Reason string `json:"reason"`
}

// Moderator 内容审核接口.
type Moderator interface {
	// Moderate 对单段文本进行审核.
	Moderate(ctx context.Context, text string) (*Result, error)
	// ModerateMessages 对消息列表进行审核.
	ModerateMessages(ctx context.Context, messages []llm.Message) (*Result, error)
}

// options 审核器内部选项.
type options struct {
	// threshold 触发标记的分数阈值，默认 0.7.
	threshold float64
	// categories 待检测的类别列表，为空则检测全部.
	categories []Category
}

// defaultOptions 返回默认选项.
func defaultOptions() *options {
	return &options{
		threshold:  0.7,
		categories: AllCategories,
	}
}

// Option 选项函数.
type Option func(*options)

// WithThreshold 设置触发标记的分数阈值（默认 0.7）.
func WithThreshold(t float64) Option {
	return func(o *options) {
		o.threshold = t
	}
}

// WithCategories 设置需要检测的类别子集.
func WithCategories(cats ...Category) Option {
	return func(o *options) {
		o.categories = cats
	}
}

var (
	// ErrNilModel 模型为 nil 时返回.
	ErrNilModel = errors.New("moderation: model is nil")
	// ErrEmptyText 文本为空时返回.
	ErrEmptyText = errors.New("moderation: empty text")
)

// ── LLM 审核器 ──────────────────────────────────────────────────────────────

// llmModerator 基于 ChatModel 的内容审核器.
type llmModerator struct {
	model llm.ChatModel
	opts  *options
}

// NewLLMModerator 创建基于 LLM 的审核器.
// model 不能为 nil，否则 Moderate 调用时返回 ErrNilModel.
func NewLLMModerator(model llm.ChatModel, opts ...Option) Moderator {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &llmModerator{model: model, opts: o}
}

// Moderate 使用 LLM 对文本进行审核.
func (m *llmModerator) Moderate(ctx context.Context, text string) (*Result, error) {
	if m.model == nil {
		return nil, ErrNilModel
	}
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmptyText
	}

	// 构造类别列表描述.
	catNames := make([]string, 0, len(m.opts.categories))
	for _, c := range m.opts.categories {
		catNames = append(catNames, string(c))
	}

	// 构造 JSON 示例，帮助 LLM 理解输出格式.
	exampleParts := make([]string, 0, len(m.opts.categories))
	for _, c := range m.opts.categories {
		exampleParts = append(exampleParts, fmt.Sprintf("%q: 0.0", string(c)))
	}
	exampleJSON := "{" + strings.Join(exampleParts, ", ") + "}"

	sysPrompt := fmt.Sprintf(
		"对以下文本进行内容审核，检查以下类别：%s。输出 JSON: {\"categories\": %s, \"reason\": \"...\"}",
		strings.Join(catNames, "、"),
		exampleJSON,
	)

	messages := []llm.Message{
		llm.SystemMessage(sysPrompt),
		llm.UserMessage(text),
	}

	resp, err := m.model.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("moderation: 模型调用失败: %w", err)
	}

	return m.parseResponse(resp.Message.Content)
}

// ModerateMessages 将消息列表的全部内容拼接后进行审核.
func (m *llmModerator) ModerateMessages(ctx context.Context, messages []llm.Message) (*Result, error) {
	parts := make([]string, 0, len(messages))
	for _, msg := range messages {
		if msg.Content != "" {
			parts = append(parts, msg.Content)
		}
	}
	return m.Moderate(ctx, strings.Join(parts, "\n"))
}

// llmResponse LLM 返回的 JSON 结构.
type llmResponse struct {
	Categories map[string]float64 `json:"categories"`
	Reason     string             `json:"reason"`
}

// parseResponse 解析 LLM 响应 JSON，生成 Result.
func (m *llmModerator) parseResponse(content string) (*Result, error) {
	content = llm.ExtractJSON(content)

	var raw llmResponse
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("moderation: JSON 解析失败: %w", err)
	}

	result := &Result{
		Categories: make(map[Category]bool),
		Scores:     make(map[Category]float64),
		Reason:     raw.Reason,
	}

	// 仅处理配置的类别.
	for _, cat := range m.opts.categories {
		score := raw.Categories[string(cat)]
		result.Scores[cat] = score
		if score > m.opts.threshold {
			result.Categories[cat] = true
			result.Flagged = true
		} else {
			result.Categories[cat] = false
		}
	}

	return result, nil
}

// ── 关键词审核器 ─────────────────────────────────────────────────────────────

// keywordModerator 基于关键词匹配的快速审核器.
type keywordModerator struct {
	// rules 各类别对应的小写关键词列表.
	rules map[Category][]string
}

// NewKeywordModerator 创建关键词审核器.
// rules 为各类别对应的关键词列表，匹配大小写不敏感.
func NewKeywordModerator(rules map[Category][]string) Moderator {
	// 将所有关键词预处理为小写，避免每次检查时重复转换.
	lowered := make(map[Category][]string, len(rules))
	for cat, keywords := range rules {
		lower := make([]string, len(keywords))
		for i, kw := range keywords {
			lower[i] = strings.ToLower(kw)
		}
		lowered[cat] = lower
	}
	return &keywordModerator{rules: lowered}
}

// Moderate 对文本进行关键词匹配审核.
func (m *keywordModerator) Moderate(_ context.Context, text string) (*Result, error) {
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmptyText
	}

	lower := strings.ToLower(text)
	result := &Result{
		Categories: make(map[Category]bool),
		Scores:     make(map[Category]float64),
	}

	for cat, keywords := range m.rules {
		matched := false
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				matched = true
				break
			}
		}
		result.Categories[cat] = matched
		if matched {
			result.Scores[cat] = 1.0
			result.Flagged = true
		} else {
			result.Scores[cat] = 0.0
		}
	}

	return result, nil
}

// ModerateMessages 将消息列表的全部内容拼接后进行关键词审核.
func (m *keywordModerator) ModerateMessages(ctx context.Context, messages []llm.Message) (*Result, error) {
	parts := make([]string, 0, len(messages))
	for _, msg := range messages {
		if msg.Content != "" {
			parts = append(parts, msg.Content)
		}
	}
	return m.Moderate(ctx, strings.Join(parts, "\n"))
}

// ── 组合审核器 ───────────────────────────────────────────────────────────────

// compositeModerator 按顺序链式调用多个审核器，支持短路.
type compositeModerator struct {
	moderators []Moderator
}

// NewCompositeModerator 创建组合审核器.
// 审核器按传入顺序依次执行；若 KeywordModerator 触发标记则短路，跳过后续（LLM）审核器.
func NewCompositeModerator(moderators ...Moderator) Moderator {
	return &compositeModerator{moderators: moderators}
}

// Moderate 依次调用各审核器，合并结果（取各类别最高分）.
func (m *compositeModerator) Moderate(ctx context.Context, text string) (*Result, error) {
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmptyText
	}

	var merged *Result

	for _, mod := range m.moderators {
		// 若已有关键词审核触发了标记，跳过后续非关键词审核器（即 LLM 审核器）.
		if merged != nil && merged.Flagged {
			if _, ok := mod.(*keywordModerator); !ok {
				continue
			}
		}

		result, err := mod.Moderate(ctx, text)
		if err != nil {
			return nil, err
		}

		merged = mergeResults(merged, result)
	}

	if merged == nil {
		// 没有任何审核器，返回空结果.
		merged = &Result{
			Categories: make(map[Category]bool),
			Scores:     make(map[Category]float64),
		}
	}

	return merged, nil
}

// ModerateMessages 依次调用各审核器对消息列表进行审核，合并结果.
func (m *compositeModerator) ModerateMessages(ctx context.Context, messages []llm.Message) (*Result, error) {
	parts := make([]string, 0, len(messages))
	for _, msg := range messages {
		if msg.Content != "" {
			parts = append(parts, msg.Content)
		}
	}
	return m.Moderate(ctx, strings.Join(parts, "\n"))
}

// mergeResults 合并两个 Result，各类别取最高分，Flagged 取 OR，Reason 拼接.
func mergeResults(base, next *Result) *Result {
	if base == nil {
		return next
	}

	merged := &Result{
		Flagged:    base.Flagged || next.Flagged,
		Categories: make(map[Category]bool),
		Scores:     make(map[Category]float64),
	}

	// 合并 base 的分数.
	for cat, score := range base.Scores {
		merged.Scores[cat] = score
		merged.Categories[cat] = base.Categories[cat]
	}

	// 合并 next 的分数，取最高值.
	for cat, score := range next.Scores {
		if existing, ok := merged.Scores[cat]; !ok || score > existing {
			merged.Scores[cat] = score
			merged.Categories[cat] = next.Categories[cat]
		}
	}

	// 拼接 Reason.
	switch {
	case base.Reason != "" && next.Reason != "":
		merged.Reason = base.Reason + "; " + next.Reason
	case base.Reason != "":
		merged.Reason = base.Reason
	default:
		merged.Reason = next.Reason
	}

	return merged
}
