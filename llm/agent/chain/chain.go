// Package chain 提供多步 AI 编排链，将多个 prompt/模型调用串联执行.
package chain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/prompt"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// 链执行错误.
var (
	// ErrNoModel 未配置默认模型.
	ErrNoModel = errors.New("chain: no model configured")
	// ErrNoSteps 未添加任何步骤.
	ErrNoSteps = errors.New("chain: no steps added")
	// ErrNilPrompt 步骤的 Prompt 为 nil.
	ErrNilPrompt = errors.New("chain: step prompt is nil")
)

// Step 单个步骤.
type Step struct {
	// Name 步骤名称.
	Name string
	// Prompt 用 input 渲染 prompt.
	Prompt *prompt.Template
	// Model 可选，覆盖默认模型.
	Model llm.ChatModel
	// Parser 可选，解析输出.
	Parser func(response string) (any, error)
}

// StepEvent 步骤事件，步骤执行完成后触发.
type StepEvent struct {
	// StepName 步骤名称.
	StepName string
	// StepIndex 步骤索引（从 0 开始）.
	StepIndex int
	// Input 步骤输入.
	Input any
	// Output 模型原始输出.
	Output string
	// Parsed Parser 解析后的结果（无 Parser 时为 nil）.
	Parsed any
	// Duration 步骤耗时.
	Duration time.Duration
}

// StepHandler 步骤回调.
type StepHandler func(ctx context.Context, event StepEvent)

// StepResult 步骤结果.
type StepResult struct {
	// Name 步骤名称.
	Name string
	// Output 模型原始输出.
	Output string
	// Parsed Parser 解析后的结果（无 Parser 时为 nil）.
	Parsed any
	// Duration 步骤耗时.
	Duration time.Duration
	// Usage token 用量统计.
	Usage llm.Usage
}

// Result 链执行结果.
type Result struct {
	// Output 最后一步的模型原始输出.
	Output string
	// Parsed 最后一步 Parser 解析后的结果（最后一步无 Parser 时为 nil）.
	Parsed any
	// Steps 各步骤结果列表.
	Steps []StepResult
	// Usage 所有步骤累计的 token 用量.
	Usage llm.Usage
}

// Option 选项函数.
type Option func(*Chain)

// WithModel 设置链的默认模型.
func WithModel(model llm.ChatModel) Option {
	return func(c *Chain) { c.model = model }
}

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option {
	return func(c *Chain) { c.log = logger.WithComponent(log, "Agent") }
}

// WithOnStep 设置步骤完成后的回调.
func WithOnStep(fn StepHandler) Option {
	return func(c *Chain) { c.onStep = fn }
}

// Chain 多步编排链.
type Chain struct {
	// model 链的默认模型.
	model llm.ChatModel
	// log 日志记录器.
	log logger.Logger
	// onStep 步骤回调.
	onStep StepHandler
	// steps 步骤列表.
	steps []Step
}

// New 创建编排链.
func New(opts ...Option) *Chain {
	c := &Chain{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// AddStep 追加一个步骤，支持链式调用.
func (c *Chain) AddStep(step Step) *Chain {
	c.steps = append(c.steps, step)
	return c
}

// Run 按顺序执行所有步骤，返回最终结果.
//
// 执行规则：
//   - 第一步直接用 input 渲染 Prompt.
//   - 后续步骤用 map{"Input": 原始 input, "Previous": 上一步输出} 渲染 Prompt.
//   - 若步骤配置了 Parser，解析结果作为下一步的输入；否则原始字符串输出作为下一步输入.
//   - 若步骤未配置 Model，使用链的默认模型.
func (c *Chain) Run(ctx context.Context, input any, opts ...llm.CallOption) (*Result, error) {
	// 校验
	if len(c.steps) == 0 {
		return nil, ErrNoSteps
	}
	for i, s := range c.steps {
		if s.Prompt == nil {
			return nil, fmt.Errorf("%w: step[%d] %q", ErrNilPrompt, i, s.Name)
		}
	}

	var (
		totalUsage  llm.Usage
		stepResults = make([]StepResult, 0, len(c.steps))
		// currentInput 当前步骤的输入（第一步为 input，后续步骤为解析结果或原始输出）
		currentInput any = input
		// lastOutput 上一步的原始字符串输出（供下一步构建 Previous 字段）
		lastOutput string
	)

	for i, step := range c.steps {
		// 确定本步骤使用的模型
		model := step.Model
		if model == nil {
			model = c.model
		}
		if model == nil {
			return nil, ErrNoModel
		}

		// 构建渲染数据：第一步直接使用 input；后续步骤传入包含 Input 和 Previous 的 map
		var renderData any
		if i == 0 {
			renderData = currentInput
		} else {
			renderData = map[string]any{
				"Input":    input,
				"Previous": lastOutput,
			}
		}

		// 渲染 Prompt
		msg, err := step.Prompt.Render(renderData)
		if err != nil {
			return nil, fmt.Errorf("chain: 步骤 %q 渲染 prompt 失败: %w", step.Name, err)
		}

		// 调用模型
		start := time.Now()
		resp, err := model.Generate(ctx, []llm.Message{msg}, opts...)
		if err != nil {
			return nil, fmt.Errorf("chain: 步骤 %q 调用模型失败: %w", step.Name, err)
		}
		duration := time.Since(start)

		output := resp.Message.Content

		// 解析输出
		var parsed any
		if step.Parser != nil {
			parsed, err = step.Parser(output)
			if err != nil {
				return nil, fmt.Errorf("chain: 步骤 %q 解析输出失败: %w", step.Name, err)
			}
			// 解析结果作为下一步输入
			currentInput = parsed
		} else {
			// 原始字符串作为下一步输入
			currentInput = output
		}

		lastOutput = output
		totalUsage.Add(resp.Usage)

		sr := StepResult{
			Name:     step.Name,
			Output:   output,
			Parsed:   parsed,
			Duration: duration,
			Usage:    resp.Usage,
		}
		stepResults = append(stepResults, sr)

		// 记录日志
		if c.log != nil {
			c.log.With(
				logger.String("step", step.Name),
				logger.Duration("duration", duration),
				logger.Int("tokens", resp.Usage.TotalTokens),
			).Info("链式步骤完成")
		}

		// 触发步骤回调
		if c.onStep != nil {
			c.onStep(ctx, StepEvent{
				StepName:  step.Name,
				StepIndex: i,
				Input:     renderData,
				Output:    output,
				Parsed:    parsed,
				Duration:  duration,
			})
		}
	}

	last := stepResults[len(stepResults)-1]
	return &Result{
		Output: last.Output,
		Parsed: last.Parsed,
		Steps:  stepResults,
		Usage:  totalUsage,
	}, nil
}
