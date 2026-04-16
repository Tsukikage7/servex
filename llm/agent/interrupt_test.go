package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/agent/checkpoint"
	"github.com/Tsukikage7/servex/v2/llm/agent/toolcall"
)

// TestToolNamePolicy 验证 ToolNamePolicy 只对指定工具名返回 true.
func TestToolNamePolicy(t *testing.T) {
	policy := ToolNamePolicy{Names: []string{"dangerous_tool", "delete_all"}}
	ctx := context.Background()

	cases := []struct {
		name     string
		toolName string
		want     bool
	}{
		{"命中第一个", "dangerous_tool", true},
		{"命中第二个", "delete_all", true},
		{"未命中", "safe_tool", false},
		{"空名称", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := policy.ShouldInterrupt(ctx, llm.ToolCall{
				Function: struct{ Name, Arguments string }{Name: tc.toolName},
			})
			if got != tc.want {
				t.Fatalf("ToolNamePolicy.ShouldInterrupt(%q) = %v, 期望 %v", tc.toolName, got, tc.want)
			}
		})
	}
}

// TestAlwaysInterruptPolicy 验证 AlwaysInterruptPolicy 对所有工具调用返回 true.
func TestAlwaysInterruptPolicy(t *testing.T) {
	policy := AlwaysInterruptPolicy{}
	ctx := context.Background()

	toolCalls := []llm.ToolCall{
		{Function: struct{ Name, Arguments string }{Name: "any_tool"}},
		{Function: struct{ Name, Arguments string }{Name: "another_tool"}},
		{},
	}
	for _, tc := range toolCalls {
		if !policy.ShouldInterrupt(ctx, tc) {
			t.Fatalf("AlwaysInterruptPolicy 应对所有工具返回 true, 实际对 %q 返回 false", tc.Function.Name)
		}
	}
}

// TestInterruptError_Error 验证 InterruptError.Error() 格式.
func TestInterruptError_Error(t *testing.T) {
	err := &InterruptError{
		CheckpointID: "cp-abc",
		ToolCalls: []llm.ToolCall{
			{Function: struct{ Name, Arguments string }{Name: "tool1"}},
			{Function: struct{ Name, Arguments string }{Name: "tool2"}},
		},
	}
	msg := err.Error()
	if msg == "" {
		t.Fatal("InterruptError.Error() 不应返回空字符串")
	}
	// 确保包含检查点 ID
	if !contains(msg, "cp-abc") {
		t.Fatalf("error message 应包含 checkpoint ID, 实际: %q", msg)
	}
}

// TestMemoryCheckpointStore 测试 MemoryStore 基本操作.
func TestMemoryCheckpointStore(t *testing.T) {
	store := checkpoint.NewMemoryStore()
	ctx := context.Background()

	cp := &checkpoint.AgentCheckpoint{
		ID: "test-cp-1",
		Messages: []llm.Message{
			llm.UserMessage("测试消息"),
		},
		ToolCalls: []llm.ToolCall{
			{ID: "tc1", Function: struct{ Name, Arguments string }{Name: "my_tool", Arguments: `{}`}},
		},
		Iteration: 1,
		Metadata:  map[string]any{"key": "value"},
	}

	// Save
	if err := store.Save(ctx, cp); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	// Load
	loaded, err := store.Load(ctx, "test-cp-1")
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if loaded.ID != cp.ID {
		t.Fatalf("ID 不匹配")
	}

	// Delete
	if err := store.Delete(ctx, "test-cp-1"); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}

	// Load after delete
	_, err = store.Load(ctx, "test-cp-1")
	if err == nil {
		t.Fatal("删除后 Load 应失败")
	}
}

// TestAgent_InterruptAndResume 测试中断与恢复完整流程.
func TestAgent_InterruptAndResume(t *testing.T) {
	// 第 1 轮：模型返回工具调用（将触发中断）
	// 第 2 轮（Resume 后）：模型返回最终回复
	model := &mockModel{
		responses: []*llm.ChatResponse{
			{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{
							ID:       "call_interrupt_1",
							Function: struct{ Name, Arguments string }{Name: "sensitive_tool", Arguments: `{"action":"delete"}`},
						},
					},
				},
				FinishReason: "tool_calls",
			},
			{
				Message:      llm.AssistantMessage("操作已完成（经人工审批）"),
				FinishReason: "stop",
			},
		},
	}

	// 注册工具
	registry := toolcall.NewRegistry()
	registry.Register(
		llm.Tool{
			Function: llm.FunctionDef{
				Name:        "sensitive_tool",
				Description: "敏感操作工具",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"action":{"type":"string"}}}`),
			},
		},
		func(_ context.Context, _ string) (string, error) {
			return `{"status":"executed"}`, nil
		},
	)

	store := checkpoint.NewMemoryStore()

	agent, err := New(&Config{
		Name:            "interrupt-agent",
		Model:           model,
		Tools:           registry,
		CheckpointStore: store,
		InterruptPolicy: AlwaysInterruptPolicy{},
	})
	if err != nil {
		t.Fatalf("创建 Agent 失败: %v", err)
	}

	// Step 1: Run → 期望返回 InterruptError
	_, runErr := agent.Run(t.Context(), "执行敏感操作")
	if runErr == nil {
		t.Fatal("期望返回 InterruptError，但 Run 成功完成")
	}

	var intErr *InterruptError
	if !errors.As(runErr, &intErr) {
		t.Fatalf("期望 *InterruptError, 实际: %T: %v", runErr, runErr)
	}
	if intErr.CheckpointID == "" {
		t.Fatal("CheckpointID 不应为空")
	}
	if len(intErr.ToolCalls) == 0 {
		t.Fatal("ToolCalls 不应为空")
	}

	cpID := intErr.CheckpointID
	t.Logf("中断检查点 ID: %s", cpID)

	// 验证检查点确实保存了
	cp, err := store.Load(t.Context(), cpID)
	if err != nil {
		t.Fatalf("加载检查点失败: %v", err)
	}
	if len(cp.ToolCalls) == 0 {
		t.Fatal("检查点应包含工具调用")
	}

	// Step 2: Resume → 审批通过
	approvals := map[string]ToolApproval{
		"call_interrupt_1": {
			Approved: true,
			Reason:   "人工审批通过",
		},
	}

	result, err := agent.Resume(t.Context(), cpID, approvals)
	if err != nil {
		t.Fatalf("Resume 失败: %v", err)
	}
	if result.Output != "操作已完成（经人工审批）" {
		t.Fatalf("Resume 输出不匹配, 期望 '操作已完成（经人工审批）', 实际: %q", result.Output)
	}

	// 检查点应被删除
	_, err = store.Load(t.Context(), cpID)
	if err == nil {
		t.Fatal("Resume 后检查点应被删除")
	}
}

// TestAgent_InterruptAndResume_Rejected 测试中断后拒绝工具调用.
func TestAgent_InterruptAndResume_Rejected(t *testing.T) {
	model := &mockModel{
		responses: []*llm.ChatResponse{
			{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{
							ID:       "call_reject_1",
							Function: struct{ Name, Arguments string }{Name: "dangerous_tool", Arguments: `{}`},
						},
					},
				},
				FinishReason: "tool_calls",
			},
			{
				Message:      llm.AssistantMessage("操作已被人工拒绝，无法继续"),
				FinishReason: "stop",
			},
		},
	}

	registry := toolcall.NewRegistry()
	registry.Register(
		llm.Tool{Function: llm.FunctionDef{Name: "dangerous_tool", Parameters: json.RawMessage(`{}`)}},
		func(_ context.Context, _ string) (string, error) {
			return `{"done":true}`, nil
		},
	)

	store := checkpoint.NewMemoryStore()
	agent, err := New(&Config{
		Name:            "reject-agent",
		Model:           model,
		Tools:           registry,
		CheckpointStore: store,
		InterruptPolicy: ToolNamePolicy{Names: []string{"dangerous_tool"}},
	})
	if err != nil {
		t.Fatalf("创建 Agent 失败: %v", err)
	}

	_, runErr := agent.Run(t.Context(), "执行危险操作")
	var intErr *InterruptError
	if !errors.As(runErr, &intErr) {
		t.Fatalf("期望 *InterruptError, 实际: %T: %v", runErr, runErr)
	}

	// 拒绝审批
	approvals := map[string]ToolApproval{
		"call_reject_1": {
			Approved: false,
			Output:   `{"error":"rejected by security policy"}`,
			Reason:   "安全策略拒绝",
		},
	}

	result, err := agent.Resume(t.Context(), intErr.CheckpointID, approvals)
	if err != nil {
		t.Fatalf("Resume 失败: %v", err)
	}
	if result.Output == "" {
		t.Fatal("Resume 应返回模型响应")
	}
}

// contains 简单的字符串包含检查.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || findSub(s, sub))
}

func findSub(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
