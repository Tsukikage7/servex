package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/agent/conversation"
	"github.com/Tsukikage7/servex/v2/llm/agent/toolcall"
	"github.com/Tsukikage7/servex/v2/llm/safety/guardrail"
)

// ---------- Mock ----------

// mockModel 模拟聊天模型，按顺序返回预设响应.
type mockModel struct {
	responses []*llm.ChatResponse
	idx       int
}

func (m *mockModel) Generate(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
	if m.idx >= len(m.responses) {
		return nil, errors.New("mock: no more responses")
	}
	resp := m.responses[m.idx]
	m.idx++
	return resp, nil
}

func (m *mockModel) Stream(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (llm.StreamReader, error) {
	if m.idx >= len(m.responses) {
		return nil, errors.New("mock: no more responses")
	}
	resp := m.responses[m.idx]
	m.idx++
	return &mockStreamReader{resp: resp}, nil
}

// mockStreamReader 将单个 ChatResponse 包装为 StreamReader，以单 token 形式发出完整内容.
type mockStreamReader struct {
	resp *llm.ChatResponse
	sent bool
}

func (r *mockStreamReader) Recv() (llm.StreamChunk, error) {
	if r.sent {
		return llm.StreamChunk{}, io.EOF
	}
	r.sent = true
	return llm.StreamChunk{Delta: r.resp.Message.Content, FinishReason: r.resp.FinishReason}, nil
}

func (r *mockStreamReader) Response() *llm.ChatResponse {
	return r.resp
}

func (r *mockStreamReader) Close() error { return nil }

// ---------- 测试 ----------

// TestNew_Validation 验证 New 函数的参数校验.
func TestNew_Validation(t *testing.T) {
	// model 为 nil 时应返回 ErrNilModel
	_, err := New(&Config{})
	if !errors.Is(err, ErrNilModel) {
		t.Fatalf("期望 ErrNilModel, 实际: %v", err)
	}

	// 正常创建
	agent, err := New(&Config{Model: &mockModel{}})
	if err != nil {
		t.Fatalf("正常创建失败: %v", err)
	}
	if agent == nil {
		t.Fatal("agent 不应为 nil")
	}
	// 默认 MaxIterations 应为 10
	if agent.cfg.MaxIterations != 10 {
		t.Fatalf("默认 MaxIterations 应为 10, 实际: %d", agent.cfg.MaxIterations)
	}
}

// TestAgent_SimpleRun 无工具、单响应的简单执行测试.
func TestAgent_SimpleRun(t *testing.T) {
	model := &mockModel{
		responses: []*llm.ChatResponse{
			{
				Message:      llm.AssistantMessage("你好，世界！"),
				Usage:        llm.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
				FinishReason: "stop",
			},
		},
	}

	agent, err := New(&Config{
		Name:         "test-agent",
		Model:        model,
		SystemPrompt: "你是一个测试助手。",
	})
	if err != nil {
		t.Fatalf("创建 Agent 失败: %v", err)
	}

	result, err := agent.Run(t.Context(), "你好")
	if err != nil {
		t.Fatalf("Run 失败: %v", err)
	}

	if result.Output != "你好，世界！" {
		t.Fatalf("输出不匹配, 期望 '你好，世界！', 实际: %q", result.Output)
	}
	if result.Iterations != 1 {
		t.Fatalf("迭代次数不匹配, 期望 1, 实际: %d", result.Iterations)
	}
	if len(result.ToolCalls) != 0 {
		t.Fatalf("不应有工具调用, 实际: %d", len(result.ToolCalls))
	}
}

// TestAgent_WithMemory 验证消息写入记忆.
func TestAgent_WithMemory(t *testing.T) {
	model := &mockModel{
		responses: []*llm.ChatResponse{
			{
				Message:      llm.AssistantMessage("记住了！"),
				FinishReason: "stop",
			},
		},
	}

	mem := conversation.NewBufferMemory()
	agent, err := New(&Config{
		Name:   "memory-agent",
		Model:  model,
		Memory: mem,
	})
	if err != nil {
		t.Fatalf("创建 Agent 失败: %v", err)
	}

	_, err = agent.Run(t.Context(), "请记住这句话")
	if err != nil {
		t.Fatalf("Run 失败: %v", err)
	}

	msgs := mem.Messages()
	if len(msgs) != 2 {
		t.Fatalf("记忆应有 2 条消息, 实际: %d", len(msgs))
	}
	if msgs[0].Role != llm.RoleUser || msgs[0].Content != "请记住这句话" {
		t.Fatalf("第 1 条记忆不匹配: %+v", msgs[0])
	}
	if msgs[1].Role != llm.RoleAssistant || msgs[1].Content != "记住了！" {
		t.Fatalf("第 2 条记忆不匹配: %+v", msgs[1])
	}
}

// TestAgent_WithGuardrail 验证护栏拦截.
func TestAgent_WithGuardrail(t *testing.T) {
	model := &mockModel{
		responses: []*llm.ChatResponse{
			{
				Message:      llm.AssistantMessage("不应到达这里"),
				FinishReason: "stop",
			},
		},
	}

	agent, err := New(&Config{
		Name:       "guarded-agent",
		Model:      model,
		Guardrails: []guardrail.Guard{guardrail.KeywordFilter([]string{"禁止"})},
	})
	if err != nil {
		t.Fatalf("创建 Agent 失败: %v", err)
	}

	_, err = agent.Run(t.Context(), "这是一条禁止的消息")
	if err == nil {
		t.Fatal("期望被护栏拦截，但没有返回错误")
	}
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("期望 ErrBlocked, 实际: %v", err)
	}
}

// TestAgent_ReActStrategy 测试 ReAct 策略的工具调用循环.
func TestAgent_ReActStrategy(t *testing.T) {
	// 第 1 轮：模型返回工具调用
	// 第 2 轮：模型返回最终回复
	model := &mockModel{
		responses: []*llm.ChatResponse{
			{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{
							ID:       "call_1",
							Function: struct{ Name, Arguments string }{Name: "calculator", Arguments: `{"expression":"1+1"}`},
						},
					},
				},
				Usage:        llm.Usage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30},
				FinishReason: "tool_calls",
			},
			{
				Message:      llm.AssistantMessage("1+1 的结果是 2"),
				Usage:        llm.Usage{PromptTokens: 30, CompletionTokens: 8, TotalTokens: 38},
				FinishReason: "stop",
			},
		},
	}

	// 注册计算器工具
	registry := toolcall.NewRegistry()
	registry.Register(
		llm.Tool{
			Function: llm.FunctionDef{
				Name:        "calculator",
				Description: "计算数学表达式",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"expression":{"type":"string"}},"required":["expression"]}`),
			},
		},
		func(_ context.Context, args string) (string, error) {
			return `{"result": 2}`, nil
		},
	)

	agent, err := New(&Config{
		Name:     "react-agent",
		Model:    model,
		Tools:    registry,
		Strategy: NewReActStrategy(),
	})
	if err != nil {
		t.Fatalf("创建 Agent 失败: %v", err)
	}

	result, err := agent.Run(t.Context(), "计算 1+1")
	if err != nil {
		t.Fatalf("Run 失败: %v", err)
	}

	if result.Output != "1+1 的结果是 2" {
		t.Fatalf("输出不匹配, 期望 '1+1 的结果是 2', 实际: %q", result.Output)
	}
	if len(result.ToolCalls) == 0 {
		t.Fatal("应至少有一次工具调用")
	}
	if result.ToolCalls[0].Call.Function.Name != "calculator" {
		t.Fatalf("工具调用名称不匹配, 期望 'calculator', 实际: %q", result.ToolCalls[0].Call.Function.Name)
	}
}

// TestAgent_PlanExecuteStrategy 测试 PlanExecute 策略.
func TestAgent_PlanExecuteStrategy(t *testing.T) {
	// 响应序列：
	// 1. 返回计划 JSON 数组
	// 2. 执行步骤 1 的结果
	// 3. 执行步骤 2 的结果
	model := &mockModel{
		responses: []*llm.ChatResponse{
			{
				Message:      llm.AssistantMessage(`["分析需求","生成代码"]`),
				Usage:        llm.Usage{PromptTokens: 15, CompletionTokens: 10, TotalTokens: 25},
				FinishReason: "stop",
			},
			{
				Message:      llm.AssistantMessage("需求分析完成"),
				Usage:        llm.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
				FinishReason: "stop",
			},
			{
				Message:      llm.AssistantMessage("代码生成完成"),
				Usage:        llm.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
				FinishReason: "stop",
			},
		},
	}

	agent, err := New(&Config{
		Name:     "plan-agent",
		Model:    model,
		Strategy: NewPlanExecuteStrategy(),
	})
	if err != nil {
		t.Fatalf("创建 Agent 失败: %v", err)
	}

	result, err := agent.Run(t.Context(), "帮我写一个 Hello World 程序")
	if err != nil {
		t.Fatalf("Run 失败: %v", err)
	}

	// 最终输出应为最后一步的结果
	if result.Output != "代码生成完成" {
		t.Fatalf("输出不匹配, 期望 '代码生成完成', 实际: %q", result.Output)
	}
	// 迭代次数应为 2（两个步骤各一次）
	if result.Iterations != 2 {
		t.Fatalf("迭代次数不匹配, 期望 2, 实际: %d", result.Iterations)
	}
}

// TestPipeline 测试管道模式.
func TestPipeline(t *testing.T) {
	// Agent 1：翻译
	model1 := &mockModel{
		responses: []*llm.ChatResponse{
			{
				Message:      llm.AssistantMessage("Hello, World!"),
				Usage:        llm.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
				FinishReason: "stop",
			},
		},
	}

	// Agent 2：格式化
	model2 := &mockModel{
		responses: []*llm.ChatResponse{
			{
				Message:      llm.AssistantMessage("**Hello, World!**"),
				Usage:        llm.Usage{PromptTokens: 8, CompletionTokens: 4, TotalTokens: 12},
				FinishReason: "stop",
			},
		},
	}

	agent1, _ := New(&Config{
		Name:         "translator",
		Model:        model1,
		SystemPrompt: "将中文翻译为英文",
	})
	agent2, _ := New(&Config{
		Name:         "formatter",
		Model:        model2,
		SystemPrompt: "将文本格式化为 Markdown 加粗",
	})

	pipeline := NewPipeline(agent1, agent2)
	result, err := pipeline.Run(t.Context(), "你好，世界！")
	if err != nil {
		t.Fatalf("Pipeline Run 失败: %v", err)
	}

	if result.Output != "**Hello, World!**" {
		t.Fatalf("输出不匹配, 期望 '**Hello, World!**', 实际: %q", result.Output)
	}

	// 总用量应累加
	expectedTotal := 15 + 12
	if result.Usage.TotalTokens != expectedTotal {
		t.Fatalf("总用量不匹配, 期望 %d, 实际: %d", expectedTotal, result.Usage.TotalTokens)
	}
}

func TestAgent_RunStream_Simple(t *testing.T) {
	model := &mockModel{
		responses: []*llm.ChatResponse{
			{
				Message:      llm.AssistantMessage("stream output"),
				Usage:        llm.Usage{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8},
				FinishReason: "stop",
			},
		},
	}

	agent, err := New(&Config{
		Name:  "stream-agent",
		Model: model,
	})
	if err != nil {
		t.Fatalf("创建 Agent 失败: %v", err)
	}

	ch, err := agent.RunStream(t.Context(), "hello")
	if err != nil {
		t.Fatalf("RunStream 失败: %v", err)
	}

	var events []Event
	for evt := range ch {
		events = append(events, evt)
	}

	if len(events) == 0 {
		t.Fatal("should receive at least one event")
	}

	// The last event should be output
	lastEvt := events[len(events)-1]
	if lastEvt.Type != EventOutput {
		t.Fatalf("last event should be output, got %s", lastEvt.Type)
	}
	if lastEvt.Content != "stream output" {
		t.Fatalf("expected 'stream output', got %q", lastEvt.Content)
	}
}

func TestAgent_RunStream_GuardrailBlocks(t *testing.T) {
	model := &mockModel{
		responses: []*llm.ChatResponse{
			{
				Message:      llm.AssistantMessage("should not reach"),
				FinishReason: "stop",
			},
		},
	}

	agent, err := New(&Config{
		Name:       "guarded-stream",
		Model:      model,
		Guardrails: []guardrail.Guard{guardrail.KeywordFilter([]string{"blocked"})},
	})
	if err != nil {
		t.Fatalf("创建 Agent 失败: %v", err)
	}

	_, err = agent.RunStream(t.Context(), "this is blocked input")
	if err == nil {
		t.Fatal("should return error for blocked input")
	}
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected ErrBlocked, got %v", err)
	}
}

func TestPipeline_Empty(t *testing.T) {
	pipeline := NewPipeline()
	result, err := pipeline.Run(t.Context(), "hello")
	if err != nil {
		t.Fatalf("empty pipeline should succeed: %v", err)
	}
	if result.Output != "hello" {
		t.Fatalf("empty pipeline should pass through input, got %q", result.Output)
	}
}
