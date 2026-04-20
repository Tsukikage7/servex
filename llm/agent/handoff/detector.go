package handoff

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Tsukikage7/servex/v2/llm"
)

// ──────────────────────────────────────────
// CompositeDetector
// ──────────────────────────────────────────

// compositeDetector 组合多个 Detector，任一命中即视为 Should=true.
type compositeDetector struct {
	detectors []Detector
}

// NewCompositeDetector 创建组合检测器.
//
// 语义：按传入顺序依次调用每个 Detector，遇到 Should=true 即立即返回该 Signal，
// 后续 Detector 不再执行（短路）. 任一 Detector 返回非 nil error 时，
// 立即返回 (nil, err)，用于上层熔断. 若所有 Detector 均未命中，
// 返回 (&Signal{Should:false}, nil).
func NewCompositeDetector(detectors ...Detector) Detector {
	return &compositeDetector{detectors: detectors}
}

// Detect 顺序执行、短路返回第一个命中的 Signal.
func (c *compositeDetector) Detect(ctx context.Context, input DetectInput) (*Signal, error) {
	for _, d := range c.detectors {
		if d == nil {
			continue
		}
		sig, err := d.Detect(ctx, input)
		if err != nil {
			return nil, err
		}
		if sig != nil && sig.Should {
			return sig, nil
		}
	}
	return notTriggered(), nil
}

// ──────────────────────────────────────────
// KeywordDetector
// ──────────────────────────────────────────

// keywordDetector 基于关键词匹配的检测器.
type keywordDetector struct {
	keywords []string
}

// NewKeywordDetector 创建关键词检测器：Question 或 Answer 包含任一关键词即命中.
//
// 约定：
//   - 空 keywords 列表的 Detector 永远不会命中
//   - 关键词匹配大小写敏感（若需不敏感，调用方自行 lower-case 后传入）
//   - 命中 Signal.Reason=ReasonKeyword，Meta["matched"] 记录命中的关键词、Meta["source"] 记录命中来源("question"/"answer")
func NewKeywordDetector(keywords []string) Detector {
	// 拷贝切片防止外部后续修改.
	kws := make([]string, 0, len(keywords))
	for _, k := range keywords {
		if k == "" {
			continue
		}
		kws = append(kws, k)
	}
	return &keywordDetector{keywords: kws}
}

// Detect 扫描 Question 再扫描 Answer，命中任一关键词立即返回.
func (d *keywordDetector) Detect(_ context.Context, input DetectInput) (*Signal, error) {
	for _, kw := range d.keywords {
		if input.Question != "" && strings.Contains(input.Question, kw) {
			return &Signal{
				Should: true,
				Reason: ReasonKeyword,
				Meta:   map[string]string{"matched": kw, "source": "question"},
			}, nil
		}
	}
	for _, kw := range d.keywords {
		if input.Answer != "" && strings.Contains(input.Answer, kw) {
			return &Signal{
				Should: true,
				Reason: ReasonKeyword,
				Meta:   map[string]string{"matched": kw, "source": "answer"},
			}, nil
		}
	}
	return notTriggered(), nil
}

// ──────────────────────────────────────────
// LowConfidenceDetector
// ──────────────────────────────────────────

// lowConfidenceDetector 基于 RAG 召回 score 阈值的检测器.
type lowConfidenceDetector struct {
	threshold float32
}

// NewLowConfidenceDetector 创建低置信度检测器：input.LastScore < threshold 即命中.
//
// 约定：
//   - 当 input.LastScore <= 0 时认为"未提供 score"，不触发命中（避免对未接入 RAG 的调用产生误判）
//   - threshold <= 0 等同于关闭该检测器
//   - 命中 Signal.Reason=ReasonLowConfidence，Meta["score"]/["threshold"] 记录具体数值
func NewLowConfidenceDetector(threshold float32) Detector {
	return &lowConfidenceDetector{threshold: threshold}
}

// Detect 阈值比较：LastScore > 0 且 LastScore < threshold 时命中.
func (d *lowConfidenceDetector) Detect(_ context.Context, input DetectInput) (*Signal, error) {
	if d.threshold <= 0 {
		return notTriggered(), nil
	}
	if input.LastScore <= 0 {
		// 未提供 score，跳过检测.
		return notTriggered(), nil
	}
	if input.LastScore < d.threshold {
		return &Signal{
			Should: true,
			Reason: ReasonLowConfidence,
			Meta: map[string]string{
				"score":     formatFloat(input.LastScore),
				"threshold": formatFloat(d.threshold),
			},
		}, nil
	}
	return notTriggered(), nil
}

// ──────────────────────────────────────────
// RetryDetector
// ──────────────────────────────────────────

// retryDetector 基于用户重复提问次数的检测器.
type retryDetector struct {
	maxRetry int
}

// NewRetryDetector 创建重试次数检测器：input.RetryCount >= maxRetry 即命中.
//
// 约定：
//   - maxRetry <= 0 等同于关闭该检测器
//   - 命中 Signal.Reason=ReasonRetryExceeded，Meta["retry_count"]/["max_retry"] 记录数值
func NewRetryDetector(maxRetry int) Detector {
	return &retryDetector{maxRetry: maxRetry}
}

// Detect 阈值比较：RetryCount >= maxRetry 时命中.
func (d *retryDetector) Detect(_ context.Context, input DetectInput) (*Signal, error) {
	if d.maxRetry <= 0 {
		return notTriggered(), nil
	}
	if input.RetryCount >= d.maxRetry {
		return &Signal{
			Should: true,
			Reason: ReasonRetryExceeded,
			Meta: map[string]string{
				"retry_count": fmt.Sprintf("%d", input.RetryCount),
				"max_retry":   fmt.Sprintf("%d", d.maxRetry),
			},
		}, nil
	}
	return notTriggered(), nil
}

// ──────────────────────────────────────────
// LLMDetector
// ──────────────────────────────────────────

// defaultLLMDetectorPrompt 默认的 LLM 检测 system prompt.
const defaultLLMDetectorPrompt = `你是客服场景下的"人工接管"判定助手。
给定用户当前问题和对话历史，判断用户是否有以下情形之一：
  1. 明确要求转人工 / 真人客服 / 投诉
  2. 情绪激烈（愤怒、威胁、反复抱怨）
  3. AI 已经连续无法解决的问题

只输出 JSON，不要任何前缀或后缀：
{"should_handoff": true/false, "reason": "简短理由"}`

// llmDetectorResponse LLM 返回的 JSON 结构.
type llmDetectorResponse struct {
	ShouldHandoff bool   `json:"should_handoff"`
	Reason        string `json:"reason"`
}

// LLMOption LLM 检测器配置选项.
type LLMOption func(*llmDetectorOptions)

// llmDetectorOptions LLMDetector 内部配置.
type llmDetectorOptions struct {
	// systemPrompt 自定义 system prompt，空则用默认.
	systemPrompt string
	// maxTokens LLM 输出最大 token.
	maxTokens int
	// callOptions 底层模型调用选项.
	callOptions []llm.CallOption
}

// defaultLLMMaxTokens LLMDetector 默认输出上限.
const defaultLLMMaxTokens = 200

// WithLLMSystemPrompt 自定义 LLM 检测 system prompt.
// 自定义 prompt 必须让 LLM 输出 {"should_handoff": bool, "reason": string} 形式的 JSON.
func WithLLMSystemPrompt(p string) LLMOption {
	return func(o *llmDetectorOptions) { o.systemPrompt = p }
}

// WithLLMMaxTokens 设置 LLM 输出最大 token 数.
func WithLLMMaxTokens(n int) LLMOption {
	return func(o *llmDetectorOptions) { o.maxTokens = n }
}

// WithLLMCallOptions 追加底层模型调用选项（例如指定 temperature、模型名等）.
func WithLLMCallOptions(opts ...llm.CallOption) LLMOption {
	return func(o *llmDetectorOptions) { o.callOptions = append(o.callOptions, opts...) }
}

// llmDetector 基于 LLM 判定的检测器.
type llmDetector struct {
	model llm.ChatModel
	opts  *llmDetectorOptions
}

// NewLLMDetector 创建基于 LLM 判定的检测器.
//
// model 为 nil 时返回 (nil, ErrNilModel). 默认 system prompt 会让 LLM 返回
// {"should_handoff": bool, "reason": string} JSON；调用方可通过 WithLLMSystemPrompt
// 覆盖，但自定义 prompt 必须保留相同输出格式. LLM 调用失败或 JSON 解析失败时
// Detect 返回 (notTriggered, error)，以"保守不转人工"为默认行为.
func NewLLMDetector(model llm.ChatModel, opts ...LLMOption) (Detector, error) {
	if model == nil {
		return nil, ErrNilModel
	}
	o := &llmDetectorOptions{maxTokens: defaultLLMMaxTokens}
	for _, opt := range opts {
		opt(o)
	}
	return &llmDetector{model: model, opts: o}, nil
}

// Detect 调用 LLM 判定并解析 JSON.
func (d *llmDetector) Detect(ctx context.Context, input DetectInput) (*Signal, error) {
	sysText := d.opts.systemPrompt
	if sysText == "" {
		sysText = defaultLLMDetectorPrompt
	}

	// 组装用户消息：历史 + 当前问题.
	var b strings.Builder
	for _, m := range input.History {
		b.WriteString(string(m.Role))
		b.WriteString(": ")
		b.WriteString(m.Content)
		b.WriteByte('\n')
	}
	b.WriteString("user: ")
	b.WriteString(input.Question)

	messages := []llm.Message{
		llm.SystemMessage(sysText),
		llm.UserMessage(b.String()),
	}

	callOpts := make([]llm.CallOption, 0, len(d.opts.callOptions)+1)
	callOpts = append(callOpts, llm.WithMaxTokens(d.opts.maxTokens))
	callOpts = append(callOpts, d.opts.callOptions...)

	resp, err := d.model.Generate(ctx, messages, callOpts...)
	if err != nil {
		return notTriggered(), fmt.Errorf("handoff: llm detect: %w", err)
	}

	content := llm.ExtractJSON(resp.Message.Content)
	var parsed llmDetectorResponse
	if uerr := json.Unmarshal([]byte(content), &parsed); uerr != nil {
		return notTriggered(), fmt.Errorf("handoff: parse llm response: %w", uerr)
	}

	if !parsed.ShouldHandoff {
		return notTriggered(), nil
	}
	return &Signal{
		Should: true,
		Reason: ReasonLLMDetected,
		Meta:   map[string]string{"llm_reason": parsed.Reason},
	}, nil
}

// formatFloat 把 float32 转为紧凑字符串，用于写入 Meta.
func formatFloat(f float32) string {
	return fmt.Sprintf("%.4f", f)
}
