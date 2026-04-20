// Package agent 提供 AI Agent 框架，支持 ReAct/PlanExecute 策略、护栏、记忆及多 Agent 编排.
package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/agent/checkpoint"
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
	// Callbacks 回调处理器列表（可选）.
	Callbacks []AgentCallbackHandler
	// CheckpointStore 检查点存储（可选，与 InterruptPolicy 配合使用）.
	CheckpointStore checkpoint.Store
	// InterruptPolicy 中断策略（可选），设置后在工具调用前触发人工审批流程.
	InterruptPolicy InterruptPolicy
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
	// EventToken 单个 token 片段（流式逐 token 输出）.
	EventToken EventType = "token"
	// EventThinking 模型思考中（每轮流结束后发一次完整文本）.
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
	ctx, span := startRunSpan(ctx, a.cfg.Name, input)
	defer span.End()

	cb := buildAgentCallbackHandler(a.cfg.Callbacks)

	// 0. 触发 OnAgentStart
	cb.OnAgentStart(ctx, input)

	// 1. 构建消息列表
	messages := a.buildMessages(input)

	// 2. 输入护栏检查
	if err := a.runGuardrails(ctx, messages); err != nil {
		err = fmt.Errorf("%w: %v", ErrBlocked, err)
		recordAgentError(span, err)
		cb.OnAgentEnd(ctx, nil, err)
		return nil, err
	}

	// 3. 委托策略执行（通过 context 传递 callbacks 和中断配置）
	ctxWithCb := withAgentCallbacks(ctx, a.cfg.Callbacks)
	if a.cfg.InterruptPolicy != nil && a.cfg.CheckpointStore != nil {
		ctxWithCb = withInterruptConfig(ctxWithCb, &interruptConfig{
			policy:    a.cfg.InterruptPolicy,
			store:     a.cfg.CheckpointStore,
			agentName: a.cfg.Name,
		})
	}
	result, err := a.cfg.Strategy.Execute(ctxWithCb, a.cfg.Model, a.cfg.Tools, messages, a.cfg.MaxIterations, a.cfg.Logger, opts...)
	if err != nil {
		recordAgentError(span, err)
		cb.OnAgentEnd(ctx, nil, err)
		return nil, err
	}

	// 4. 输出护栏检查
	if result.Output != "" && len(a.cfg.Guardrails) > 0 {
		outMsgs := []llm.Message{llm.AssistantMessage(result.Output)}
		if err := a.runGuardrails(ctx, outMsgs); err != nil {
			err = fmt.Errorf("%w: %v", ErrBlocked, err)
			recordAgentError(span, err)
			cb.OnAgentEnd(ctx, nil, err)
			return nil, err
		}
	}

	// 5. 写入记忆
	if a.cfg.Memory != nil {
		a.cfg.Memory.Add(llm.UserMessage(input))
		a.cfg.Memory.Add(llm.AssistantMessage(result.Output))
	}

	// 6. 触发 OnAgentEnd
	recordAgentResult(span, result)
	cb.OnAgentEnd(ctx, result, nil)
	return result, nil
}

// RunStream 流式执行 Agent，通过 channel 返回事件流.
// 整体流程与 Run 一致，但策略层以事件流方式返回中间过程.
func (a *Agent) RunStream(ctx context.Context, input string, opts ...llm.CallOption) (<-chan Event, error) {
	ctx, span := startRunStreamSpan(ctx, a.cfg.Name, input)
	// 注意:此函数返回 channel,span 需要跟随流的生命周期关闭.
	// 若初始阶段出错,span 就地结束;成功后 span 随 goroutine 结束时关闭.
	spanDone := false
	defer func() {
		if !spanDone {
			span.End()
		}
	}()

	cb := buildAgentCallbackHandler(a.cfg.Callbacks)

	// 触发 OnAgentStart
	cb.OnAgentStart(ctx, input)

	// 构建消息列表
	messages := a.buildMessages(input)

	// 输入护栏检查
	if err := a.runGuardrails(ctx, messages); err != nil {
		err = fmt.Errorf("%w: %v", ErrBlocked, err)
		recordAgentError(span, err)
		cb.OnAgentEnd(ctx, nil, err)
		return nil, err
	}

	// 委托策略执行流式（通过 context 传递 callbacks）
	ctxWithCb := withAgentCallbacks(ctx, a.cfg.Callbacks)
	ch, err := a.cfg.Strategy.ExecuteStream(ctxWithCb, a.cfg.Model, a.cfg.Tools, messages, a.cfg.MaxIterations, a.cfg.Logger, opts...)
	if err != nil {
		recordAgentError(span, err)
		cb.OnAgentEnd(ctx, nil, err)
		return nil, err
	}

	// 交接:span 生命周期从此处转移到下方 goroutine.
	spanDone = true

	// 包装 channel，在输出事件上执行输出护栏并写入记忆
	outCh := make(chan Event, 16)
	go func() {
		defer span.End()
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
					guardErr := fmt.Errorf("%w: %v", ErrBlocked, gErr)
					recordAgentError(span, guardErr)
					cb.OnAgentEnd(ctx, nil, guardErr)
					outCh <- Event{Type: EventError, Content: guardErr.Error()}
					return
				}
			}
			outCh <- evt

			// 输出事件写入记忆并触发 OnAgentEnd
			if evt.Type == EventOutput {
				if a.cfg.Memory != nil {
					a.cfg.Memory.Add(llm.UserMessage(input))
					a.cfg.Memory.Add(llm.AssistantMessage(evt.Content))
				}
				cb.OnAgentEnd(ctx, &Result{Output: evt.Content}, nil)
			}
		}
	}()

	return outCh, nil
}

// Resume 从检查点恢复 Agent 执行.
// checkpointID 指定要恢复的检查点.
// approvals 为 toolCallID → 审批结果的映射；未出现在映射中的工具调用默认拒绝.
func (a *Agent) Resume(ctx context.Context, checkpointID string, approvals map[string]ToolApproval, opts ...llm.CallOption) (*Result, error) {
	if a.cfg.CheckpointStore == nil {
		return nil, errors.New("agent: CheckpointStore 未配置，无法恢复")
	}

	// 注入 callbacks（与 Run 一致）
	cb := buildAgentCallbackHandler(a.cfg.Callbacks)
	cb.OnAgentStart(ctx, "resume:"+checkpointID)
	ctx = withAgentCallbacks(ctx, a.cfg.Callbacks)

	// 注入中断配置（允许 Resume 后的执行中再次中断）
	if a.cfg.InterruptPolicy != nil {
		ctx = withInterruptConfig(ctx, &interruptConfig{
			policy:    a.cfg.InterruptPolicy,
			store:     a.cfg.CheckpointStore,
			agentName: a.cfg.Name,
		})
	}

	// 1. 加载检查点
	cp, err := a.cfg.CheckpointStore.Load(ctx, checkpointID)
	if err != nil {
		cb.OnAgentEnd(ctx, nil, err)
		return nil, fmt.Errorf("agent: 加载检查点失败: %w", err)
	}

	// 2. 根据审批结果构建工具结果消息
	messages := make([]llm.Message, len(cp.Messages))
	copy(messages, cp.Messages)

	for _, tc := range cp.ToolCalls {
		approval, ok := approvals[tc.ID]
		if !ok {
			// 未审批：默认拒绝
			messages = append(messages, llm.ToolResultMessage(tc.ID, `{"error":"not approved"}`))
			continue
		}
		if approval.Approved {
			// 审批通过：执行工具
			output := ""
			if a.cfg.Tools != nil {
				var execErr error
				output, execErr = a.cfg.Tools.Execute(ctx, tc.ID, tc.Function.Name, tc.Function.Arguments)
				if execErr != nil {
					output = fmt.Sprintf(`{"error":%q}`, execErr.Error())
				}
			} else {
				output = `{"error":"no tools registered"}`
			}
			messages = append(messages, llm.ToolResultMessage(tc.ID, output))
		} else {
			// 拒绝：使用替代输出
			output := approval.Output
			if output == "" {
				output = `{"error":"tool call rejected by human"}`
			}
			messages = append(messages, llm.ToolResultMessage(tc.ID, output))
		}
	}

	// 3. 继续 Agent 执行循环（从工具结果之后继续）
	remainingIter := a.cfg.MaxIterations - cp.Iteration
	if remainingIter <= 0 {
		remainingIter = 1
	}
	result, err := a.cfg.Strategy.Execute(ctx, a.cfg.Model, a.cfg.Tools, messages, remainingIter, a.cfg.Logger, opts...)
	if err != nil {
		cb.OnAgentEnd(ctx, nil, err)
		return nil, err
	}

	// 4. 输出护栏检查
	if result.Output != "" && len(a.cfg.Guardrails) > 0 {
		outMsgs := []llm.Message{llm.AssistantMessage(result.Output)}
		if gErr := a.runGuardrails(ctx, outMsgs); gErr != nil {
			gErr = fmt.Errorf("%w: %v", ErrBlocked, gErr)
			cb.OnAgentEnd(ctx, nil, gErr)
			return nil, gErr
		}
	}

	// 5. 写入记忆（只记录最终输出）
	if a.cfg.Memory != nil && result.Output != "" {
		a.cfg.Memory.Add(llm.AssistantMessage(result.Output))
	}

	// 6. 删除已用检查点
	_ = a.cfg.CheckpointStore.Delete(ctx, checkpointID)

	cb.OnAgentEnd(ctx, result, nil)
	return result, nil
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
