// Package toolcall 提供 AI 工具调用框架，支持工具注册和自动循环执行.
package toolcall

import (
	"context"
	"errors"
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm"
)

// ErrMaxRounds 超过最大工具调用轮次.
var ErrMaxRounds = errors.New("toolcall: 超过最大工具调用轮次")

// ExecutorOption 执行器选项.
type ExecutorOption func(*executorOptions)

// executorOptions 内部选项集合.
type executorOptions struct {
	maxRounds int
	onStep    StepHandler
}

// WithMaxRounds 设置最大工具调用轮次（默认 10）.
func WithMaxRounds(n int) ExecutorOption {
	return func(o *executorOptions) { o.maxRounds = n }
}

// ToolResult 单次工具调用的执行结果.
type ToolResult struct {
	Call   llm.ToolCall // 原始工具调用请求
	Output string       // 执行结果（JSON 字符串）
	Err    error        // 执行错误（nil 表示成功）
}

// StepEvent 单轮推理步骤事件，每轮 LLM 响应处理完毕后触发.
type StepEvent struct {
	Round       int               // 轮次序号（0-based）
	Response    *llm.ChatResponse // 本轮 LLM 响应（含 Content/ToolCalls）
	ToolResults []ToolResult      // 本轮工具执行结果（IsFinal=true 时为 nil）
	IsFinal     bool              // 是否为最终轮（模型未请求工具调用，循环即将结束）
}

// StepHandler 步骤回调函数类型.
type StepHandler func(ctx context.Context, event StepEvent)

// WithOnStep 设置每轮推理步骤的回调函数.
// 每当 LLM 返回响应后均触发一次（含最终轮）.
func WithOnStep(fn StepHandler) ExecutorOption {
	return func(o *executorOptions) { o.onStep = fn }
}

// ExecutorResult 执行结果.
type ExecutorResult struct {
	// Response 最终的模型响应（工具调用循环结束后的回复）.
	Response *llm.ChatResponse
	// Messages 完整对话历史（含所有中间工具调用和结果）.
	Messages []llm.Message
	// Rounds 实际执行的工具调用轮次数.
	Rounds int
}

// Executor 工具调用自动循环执行器.
// 自动处理 model → tool_calls → execute → result → model 的循环直到模型停止调用工具.
type Executor struct {
	model    llm.ChatModel
	registry *Registry
	opts     executorOptions
}

// NewExecutor 创建工具调用执行器.
func NewExecutor(model llm.ChatModel, registry *Registry, opts ...ExecutorOption) *Executor {
	o := executorOptions{maxRounds: 10}
	for _, opt := range opts {
		opt(&o)
	}
	return &Executor{model: model, registry: registry, opts: o}
}

// Run 执行工具调用循环.
// 自动向 opts 中注入 registry 中的工具列表.
// 循环直到：模型不再请求工具调用、达到最大轮次、或发生错误.
func (e *Executor) Run(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*ExecutorResult, error) {
	// 注入工具列表（附加在调用选项之后，不覆盖用户指定的工具）
	tools := e.registry.Tools()
	if len(tools) > 0 {
		opts = append([]llm.CallOption{llm.WithTools(tools...)}, opts...)
	}

	history := make([]llm.Message, len(messages))
	copy(history, messages)

	result := &ExecutorResult{}

	for round := range e.opts.maxRounds {
		resp, err := e.model.Generate(ctx, history, opts...)
		if err != nil {
			return nil, fmt.Errorf("toolcall: 第 %d 轮生成失败: %w", round+1, err)
		}

		// 将助手消息加入历史
		history = append(history, resp.Message)

		// 没有工具调用，循环结束
		if len(resp.Message.ToolCalls) == 0 {
			if e.opts.onStep != nil {
				e.opts.onStep(ctx, StepEvent{Round: round, Response: resp, IsFinal: true})
			}
			result.Response = resp
			result.Messages = history
			result.Rounds = round
			return result, nil
		}

		result.Rounds = round + 1

		// 执行所有工具调用，收集结果
		toolResults := make([]ToolResult, 0, len(resp.Message.ToolCalls))
		for _, tc := range resp.Message.ToolCalls {
			output, execErr := e.registry.Execute(ctx, tc.ID, tc.Function.Name, tc.Function.Arguments)
			toolResults = append(toolResults, ToolResult{Call: tc, Output: output, Err: execErr})
			if execErr != nil {
				// 将错误作为工具结果返回，让模型处理
				output = fmt.Sprintf(`{"error": %q}`, execErr.Error())
			}
			history = append(history, llm.ToolResultMessage(tc.ID, output))
		}
		// 工具调用轮 emit（所有工具执行完后统一触发一次）
		if e.opts.onStep != nil {
			e.opts.onStep(ctx, StepEvent{Round: round, Response: resp, ToolResults: toolResults})
		}
	}

	return nil, fmt.Errorf("%w: %d", ErrMaxRounds, e.opts.maxRounds)
}
