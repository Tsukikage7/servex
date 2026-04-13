// Package agent 提供 AI Agent 框架，支持 ReAct/PlanExecute 策略、护栏、记忆及多 Agent 编排.
package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/agent/conversation"
	"github.com/Tsukikage7/servex/v2/llm/agent/toolcall"
	"github.com/Tsukikage7/servex/v2/llm/safety/guardrail"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// 哨兵错误.
var (
	// ErrNilModel 模型为 nil 时返回.
	ErrNilModel = errors.New("agent: model is nil")
	// ErrMaxIterations 达到最大迭代次数时返回.
	ErrMaxIterations = errors.New("agent: max iterations reached")
	// ErrBlocked 被护栏拦截时返回.
	ErrBlocked = errors.New("agent: blocked by guardrail")
)

// Config Agent 配置.
type Config struct {
	// Name Agent 名称.
	Name string
	// Model 聊天模型（必填）.
	Model llm.ChatModel
	// SystemPrompt 系统提示词.
	SystemPrompt string
	// Tools 工具注册表（可选）.
	Tools *toolcall.Registry
	// Memory 记忆策略（可选）.
	Memory conversation.Memory
	// Guardrails 护栏列表（可选）.
	Guardrails []guardrail.Guard
	// Strategy 执行策略（默认 ReAct）.
	Strategy Strategy
	// MaxIterations 最大迭代轮次（默认 10）.
	MaxIterations int
	// Logger 日志记录器（可选）.
	Logger logger.Logger
}

// Agent 智能代理，封装模型调用、工具执行、记忆管理和护栏校验.
type Agent struct {
	cfg Config
}

// New 创建 Agent 实例.
// 校验 Model 不为 nil，并为可选字段填充默认值.
func New(cfg *Config) (*Agent, error) {
	if cfg.Model == nil {
		return nil, ErrNilModel
	}
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 10
	}
	if cfg.Strategy == nil {
		cfg.Strategy = NewReActStrategy()
	}
	return &Agent{cfg: *cfg}, nil
}

// Result Agent 执行结果.
type Result struct {
	// Output 最终输出文本.
	Output string
	// Messages 完整对话消息列表.
	Messages []llm.Message
	// ToolCalls 所有工具调用结果.
	ToolCalls []toolcall.ToolResult
	// Iterations 实际迭代轮次.
	Iterations int
	// Usage token 用量统计.
	Usage llm.Usage
}

// EventType 事件类型.
type EventType string

const (
	// EventThinking 模型思考中.
	EventThinking EventType = "thinking"
	// EventToolCall 工具调用请求.
	EventToolCall EventType = "tool_call"
	// EventToolResult 工具调用结果.
	EventToolResult EventType = "tool_result"
	// EventOutput 最终输出.
	EventOutput EventType = "output"
	// EventError 错误事件.
	EventError EventType = "error"
)

// Event 流式事件.
type Event struct {
	// Type 事件类型.
	Type EventType
	// Content 文本内容.
	Content string
	// ToolCall 工具调用信息（Type=EventToolCall 时）.
	ToolCall *llm.ToolCall
	// ToolResult 工具调用结果（Type=EventToolResult 时）.
	ToolResult *toolcall.ToolResult
}

// Run 执行 Agent，返回最终结果.
// 流程：构建消息 → 输入护栏 → 策略执行 → 输出护栏 → 写入记忆 → 返回结果.
func (a *Agent) Run(ctx context.Context, input string, opts ...llm.CallOption) (*Result, error) {
	// 1. 构建消息列表
	messages := a.buildMessages(input)

	// 2. 输入护栏检查
	if err := a.runGuardrails(ctx, messages); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBlocked, err)
	}

	// 3. 委托策略执行
	result, err := a.cfg.Strategy.Execute(ctx, a.cfg.Model, a.cfg.Tools, messages, a.cfg.MaxIterations, a.cfg.Logger, opts...)
	if err != nil {
		return nil, err
	}

	// 4. 输出护栏检查
	if result.Output != "" && len(a.cfg.Guardrails) > 0 {
		outMsgs := []llm.Message{llm.AssistantMessage(result.Output)}
		if err := a.runGuardrails(ctx, outMsgs); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBlocked, err)
		}
	}

	// 5. 写入记忆
	if a.cfg.Memory != nil {
		a.cfg.Memory.Add(llm.UserMessage(input))
		a.cfg.Memory.Add(llm.AssistantMessage(result.Output))
	}

	return result, nil
}

// RunStream 流式执行 Agent，通过 channel 返回事件流.
// 整体流程与 Run 一致，但策略层以事件流方式返回中间过程.
func (a *Agent) RunStream(ctx context.Context, input string, opts ...llm.CallOption) (<-chan Event, error) {
	// 构建消息列表
	messages := a.buildMessages(input)

	// 输入护栏检查
	if err := a.runGuardrails(ctx, messages); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBlocked, err)
	}

	// 委托策略执行流式
	ch, err := a.cfg.Strategy.ExecuteStream(ctx, a.cfg.Model, a.cfg.Tools, messages, a.cfg.MaxIterations, a.cfg.Logger, opts...)
	if err != nil {
		return nil, err
	}

	// 包装 channel，在输出事件上执行输出护栏并写入记忆
	outCh := make(chan Event, 16)
	go func() {
		defer close(outCh)
		defer func() {
			if r := recover(); r != nil {
				outCh <- Event{Type: EventError, Content: fmt.Sprintf("goroutine panic: %v", r)}
			}
		}()
		for evt := range ch {
			// 输出事件执行护栏检查
			if evt.Type == EventOutput && len(a.cfg.Guardrails) > 0 {
				outMsgs := []llm.Message{llm.AssistantMessage(evt.Content)}
				if gErr := a.runGuardrails(ctx, outMsgs); gErr != nil {
					outCh <- Event{Type: EventError, Content: fmt.Sprintf("%v: %v", ErrBlocked, gErr)}
					return
				}
			}
			outCh <- evt

			// 输出事件写入记忆
			if evt.Type == EventOutput && a.cfg.Memory != nil {
				a.cfg.Memory.Add(llm.UserMessage(input))
				a.cfg.Memory.Add(llm.AssistantMessage(evt.Content))
			}
		}
	}()

	return outCh, nil
}

// buildMessages 构建消息列表：系统提示 + 记忆消息 + 用户输入.
func (a *Agent) buildMessages(input string) []llm.Message {
	var messages []llm.Message

	// 系统提示
	if a.cfg.SystemPrompt != "" {
		messages = append(messages, llm.SystemMessage(a.cfg.SystemPrompt))
	}

	// 记忆中的历史消息
	if a.cfg.Memory != nil {
		messages = append(messages, a.cfg.Memory.Messages()...)
	}

	// 当前用户输入
	messages = append(messages, llm.UserMessage(input))
	return messages
}

// runGuardrails 执行所有护栏检查.
func (a *Agent) runGuardrails(ctx context.Context, messages []llm.Message) error {
	for _, g := range a.cfg.Guardrails {
		if err := g.Check(ctx, messages); err != nil {
			return err
		}
	}
	return nil
}
