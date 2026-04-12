// Package eval 提供 LLM 输出质量评估框架.
//
// 支持相关性、忠实性、连贯性、正确性等多维度评估，
// 每个维度由独立的 Evaluator 实现，可通过 CompositeEvaluator 组合使用.
package eval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Tsukikage7/servex/llm"
)

// 评估错误类型.
var (
	// ErrNilModel 模型为 nil.
	ErrNilModel = errors.New("evaluation: model is nil")
	// ErrEmptyAnswer 被评估答案为空.
	ErrEmptyAnswer = errors.New("evaluation: answer is empty")
	// ErrParseResponse 无法解析评估响应.
	ErrParseResponse = errors.New("evaluation: failed to parse evaluation response")
)

// Score 单项评分结果.
type Score struct {
	// Name 评估维度名称.
	Name string `json:"name"`
	// Value 分值，范围 0.0-1.0.
	Value float64 `json:"value"`
	// Reason 评分理由.
	Reason string `json:"reason"`
}

// EvalResult 评估结果，包含多个维度的评分.
type EvalResult struct {
	// Scores 各维度评分列表.
	Scores []Score `json:"scores"`
}

// EvalInput 评估输入数据.
type EvalInput struct {
	// Question 原始问题.
	Question string `json:"question"`
	// Answer 待评估的答案.
	Answer string `json:"answer"`
	// Reference 参考答案，用于正确性评估（可选）.
	Reference string `json:"reference,omitempty"`
	// Context 参考资料列表，用于忠实性评估（可选）.
	Context []string `json:"context,omitzero"`
}

// Evaluator 评估器接口.
type Evaluator interface {
	// Evaluate 对给定输入执行评估，返回评估结果.
	Evaluate(ctx context.Context, input EvalInput) (*EvalResult, error)
}

// options 评估器内部选项.
type options struct {
	// callOptions 底层模型调用选项.
	callOptions []llm.CallOption
}

// Option 评估器选项函数.
type Option func(*options)

// WithCallOptions 设置底层模型调用选项.
func WithCallOptions(opts ...llm.CallOption) Option {
	return func(o *options) {
		o.callOptions = append(o.callOptions, opts...)
	}
}

// llmEvalResponse LLM 返回的评估 JSON 结构.
type llmEvalResponse struct {
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

// llmEvaluator 基于 LLM 的评估器基础实现.
type llmEvaluator struct {
	// name 评估维度名称.
	name string
	// model 底层聊天模型.
	model llm.ChatModel
	// opts 调用选项.
	opts options
	// buildPrompt 根据输入构造系统提示的函数.
	buildPrompt func(input EvalInput) string
}

// Evaluate 执行 LLM 评估.
func (e *llmEvaluator) Evaluate(ctx context.Context, input EvalInput) (*EvalResult, error) {
	// 校验输入.
	if input.Answer == "" {
		return nil, ErrEmptyAnswer
	}

	// 构造系统提示.
	sysPrompt := e.buildPrompt(input)

	// 构造用户消息.
	userContent := fmt.Sprintf("问题：%s\n\n回答：%s", input.Question, input.Answer)

	messages := []llm.Message{
		llm.SystemMessage(sysPrompt),
		llm.UserMessage(userContent),
	}

	// 调用模型.
	resp, err := e.model.Generate(ctx, messages, e.opts.callOptions...)
	if err != nil {
		return nil, fmt.Errorf("evaluation: 模型调用失败: %w", err)
	}

	// 解析 JSON 响应.
	content := llm.ExtractJSON(resp.Message.Content)
	var evalResp llmEvalResponse
	if err := json.Unmarshal([]byte(content), &evalResp); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParseResponse, err)
	}

	// 归一化分值到 [0, 1].
	score := clamp(evalResp.Score, 0.0, 1.0)

	return &EvalResult{
		Scores: []Score{
			{
				Name:   e.name,
				Value:  score,
				Reason: evalResp.Reason,
			},
		},
	}, nil
}

// RelevanceEvaluator 创建相关性评估器，评估回答与问题的相关程度.
func RelevanceEvaluator(model llm.ChatModel, opts ...Option) Evaluator {
	if model == nil {
		return &errEvaluator{err: ErrNilModel}
	}
	o := applyOptions(opts)
	return &llmEvaluator{
		name:  "relevance",
		model: model,
		opts:  o,
		buildPrompt: func(_ EvalInput) string {
			return `你是一位专业的 AI 评估专家。评估回答与问题的相关程度。` +
				`输出JSON: {"score": 0-1, "reason": "..."}`
		},
	}
}

// FaithfulnessEvaluator 创建忠实性评估器，评估回答是否忠实于参考资料.
func FaithfulnessEvaluator(model llm.ChatModel, opts ...Option) Evaluator {
	if model == nil {
		return &errEvaluator{err: ErrNilModel}
	}
	o := applyOptions(opts)
	return &llmEvaluator{
		name:  "faithfulness",
		model: model,
		opts:  o,
		buildPrompt: func(input EvalInput) string {
			ctx := strings.Join(input.Context, "\n")
			return fmt.Sprintf(
				`你是一位专业的 AI 评估专家。评估回答是否忠实于给定的参考资料，不包含虚构内容。`+
					`参考资料：%s。输出JSON: {"score": 0-1, "reason": "..."}`,
				ctx,
			)
		},
	}
}

// CoherenceEvaluator 创建连贯性评估器，评估回答的逻辑连贯性和可读性.
func CoherenceEvaluator(model llm.ChatModel, opts ...Option) Evaluator {
	if model == nil {
		return &errEvaluator{err: ErrNilModel}
	}
	o := applyOptions(opts)
	return &llmEvaluator{
		name:  "coherence",
		model: model,
		opts:  o,
		buildPrompt: func(_ EvalInput) string {
			return `你是一位专业的 AI 评估专家。评估回答的逻辑连贯性和可读性。` +
				`输出JSON: {"score": 0-1, "reason": "..."}`
		},
	}
}

// CorrectnessEvaluator 创建正确性评估器，评估回答与参考答案的一致程度.
func CorrectnessEvaluator(model llm.ChatModel, opts ...Option) Evaluator {
	if model == nil {
		return &errEvaluator{err: ErrNilModel}
	}
	o := applyOptions(opts)
	return &llmEvaluator{
		name:  "correctness",
		model: model,
		opts:  o,
		buildPrompt: func(input EvalInput) string {
			return fmt.Sprintf(
				`你是一位专业的 AI 评估专家。评估回答与参考答案的一致程度。`+
					`参考答案：%s。输出JSON: {"score": 0-1, "reason": "..."}`,
				input.Reference,
			)
		},
	}
}

// compositeEvaluator 组合评估器，并发运行多个评估器并合并结果.
type compositeEvaluator struct {
	evaluators []Evaluator
}

// NewCompositeEvaluator 创建组合评估器，并发运行所有子评估器并合并评分.
func NewCompositeEvaluator(evaluators ...Evaluator) Evaluator {
	return &compositeEvaluator{evaluators: evaluators}
}

// Evaluate 并发运行所有子评估器，合并所有 Score.
// 若任一子评估器返回错误，则整体返回第一个遇到的错误.
func (c *compositeEvaluator) Evaluate(ctx context.Context, input EvalInput) (*EvalResult, error) {
	type result struct {
		scores []Score
		err    error
	}

	results := make([]result, len(c.evaluators))
	var wg sync.WaitGroup

	for i, ev := range c.evaluators {
		wg.Go(func() {
			r, err := ev.Evaluate(ctx, input)
			if err != nil {
				results[i] = result{err: err}
				return
			}
			results[i] = result{scores: r.Scores}
		})
	}

	wg.Wait()

	// 收集所有 Score，遇到错误立即返回.
	var allScores []Score
	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
		allScores = append(allScores, r.scores...)
	}

	return &EvalResult{Scores: allScores}, nil
}

// errEvaluator 始终返回固定错误的评估器，用于构造时参数校验失败的情况.
type errEvaluator struct {
	err error
}

// Evaluate 始终返回构造时的错误.
func (e *errEvaluator) Evaluate(_ context.Context, _ EvalInput) (*EvalResult, error) {
	return nil, e.err
}

// applyOptions 应用选项列表，返回合并后的选项.
func applyOptions(opts []Option) options {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// clamp 将 v 限制在 [min, max] 范围内.
func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
