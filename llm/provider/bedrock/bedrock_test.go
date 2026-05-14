package bedrock

import (
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/Tsukikage7/servex/v2/llm"
)

// ─── New / 构造 ──────────────────────────────────────────────────────────────

func TestNew_DefaultConfig(t *testing.T) {
	c := New(nil)
	if c == nil {
		t.Fatal("New() 返回 nil")
	}
	if c.model != "anthropic.claude-3-5-sonnet-20241022-v2:0" {
		t.Errorf("默认 model = %q，期望 anthropic.claude-3-5-sonnet-20241022-v2:0", c.model)
	}
}

func TestNew_WithModel(t *testing.T) {
	c := New(nil, WithModel("amazon.titan-text-express-v1"))
	if c.model != "amazon.titan-text-express-v1" {
		t.Errorf("model = %q，期望 amazon.titan-text-express-v1", c.model)
	}
}

// TestNew_InterfaceCompliance 若编译通过即 pass.
func TestNew_InterfaceCompliance(t *testing.T) {
	var _ llm.ChatModel = (*Client)(nil)
}

// ─── convertMessages / splitMessages ────────────────────────────────────────

func TestConvertMessages_SystemSeparation(t *testing.T) {
	messages := []llm.Message{
		llm.SystemMessage("你是一个助手"),
		llm.UserMessage("你好"),
		llm.AssistantMessage("您好！"),
	}

	sys, conv := splitMessages(messages)

	if len(sys) != 1 {
		t.Fatalf("system blocks = %d，期望 1", len(sys))
	}
	sb, ok := sys[0].(*types.SystemContentBlockMemberText)
	if !ok {
		t.Fatal("system block 不是 *types.SystemContentBlockMemberText")
	}
	if sb.Value != "你是一个助手" {
		t.Errorf("system text = %q，期望 %q", sb.Value, "你是一个助手")
	}

	if len(conv) != 2 {
		t.Fatalf("conv messages = %d，期望 2", len(conv))
	}
	if conv[0].Role != types.ConversationRoleUser {
		t.Errorf("conv[0].Role = %v，期望 user", conv[0].Role)
	}
	if conv[1].Role != types.ConversationRoleAssistant {
		t.Errorf("conv[1].Role = %v，期望 assistant", conv[1].Role)
	}
}

func TestConvertMessages_RoleMapping(t *testing.T) {
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: "用户消息"},
		{Role: llm.RoleAssistant, Content: "助手消息"},
	}

	_, conv := splitMessages(messages)
	if len(conv) != 2 {
		t.Fatalf("conv = %d，期望 2", len(conv))
	}
	if conv[0].Role != types.ConversationRoleUser {
		t.Error("user role mapping 失败")
	}
	if conv[1].Role != types.ConversationRoleAssistant {
		t.Error("assistant role mapping 失败")
	}
}

func TestConvertMessages_ToolCall(t *testing.T) {
	msg := llm.Message{
		Role:    llm.RoleAssistant,
		Content: "",
		ToolCalls: []llm.ToolCall{
			{ID: "call_abc", Function: struct {
				Name      string
				Arguments string
			}{Name: "get_weather", Arguments: `{"location":"Beijing"}`}},
		},
	}

	_, conv := splitMessages([]llm.Message{msg})
	if len(conv) != 1 {
		t.Fatalf("conv = %d，期望 1", len(conv))
	}

	found := false
	for _, block := range conv[0].Content {
		tu, ok := block.(*types.ContentBlockMemberToolUse)
		if !ok {
			continue
		}
		if *tu.Value.ToolUseId != "call_abc" {
			t.Errorf("ToolUseId = %q，期望 call_abc", *tu.Value.ToolUseId)
		}
		if *tu.Value.Name != "get_weather" {
			t.Errorf("Name = %q，期望 get_weather", *tu.Value.Name)
		}
		found = true
	}
	if !found {
		t.Error("未找到 ToolUseBlock")
	}
}

func TestConvertMessages_ToolResult(t *testing.T) {
	msg := llm.ToolResultMessage("call_abc", "晴天，25°C")

	_, conv := splitMessages([]llm.Message{msg})
	if len(conv) != 1 {
		t.Fatalf("conv = %d，期望 1", len(conv))
	}
	if conv[0].Role != types.ConversationRoleUser {
		t.Error("tool result 消息应映射为 user 角色")
	}

	found := false
	for _, block := range conv[0].Content {
		tr, ok := block.(*types.ContentBlockMemberToolResult)
		if !ok {
			continue
		}
		if *tr.Value.ToolUseId != "call_abc" {
			t.Errorf("ToolUseId = %q，期望 call_abc", *tr.Value.ToolUseId)
		}
		if len(tr.Value.Content) != 1 {
			t.Fatalf("ToolResult.Content = %d，期望 1", len(tr.Value.Content))
		}
		txt, ok2 := tr.Value.Content[0].(*types.ToolResultContentBlockMemberText)
		if !ok2 {
			t.Fatal("content block 不是 text")
		}
		if txt.Value != "晴天，25°C" {
			t.Errorf("content = %q，期望 晴天，25°C", txt.Value)
		}
		found = true
	}
	if !found {
		t.Error("未找到 ToolResultBlock")
	}
}

// ─── convertResponse ────────────────────────────────────────────────────────

func TestConvertResponse_TextMessage(t *testing.T) {
	inputTokens := int32(10)
	outputTokens := int32(20)
	totalTokens := int32(30)

	resp := &bedrockruntime.ConverseOutput{
		StopReason: types.StopReasonEndTurn,
		Usage: &types.TokenUsage{
			InputTokens:  &inputTokens,
			OutputTokens: &outputTokens,
			TotalTokens:  &totalTokens,
		},
		Output: &types.ConverseOutputMemberMessage{
			Value: types.Message{
				Role: types.ConversationRoleAssistant,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{Value: "这是助手的回复"},
				},
			},
		},
	}

	result := convertResponse(resp)

	if result.FinishReason != "end_turn" {
		t.Errorf("FinishReason = %q，期望 end_turn", result.FinishReason)
	}
	if result.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d，期望 10", result.Usage.PromptTokens)
	}
	if result.Usage.CompletionTokens != 20 {
		t.Errorf("CompletionTokens = %d，期望 20", result.Usage.CompletionTokens)
	}
	if result.Usage.TotalTokens != 30 {
		t.Errorf("TotalTokens = %d，期望 30", result.Usage.TotalTokens)
	}
	if result.Message.Role != llm.RoleAssistant {
		t.Errorf("Role = %v，期望 assistant", result.Message.Role)
	}
	if result.Message.Content != "这是助手的回复" {
		t.Errorf("Content = %q，期望 这是助手的回复", result.Message.Content)
	}
}

func TestConvertResponse_ToolUse(t *testing.T) {
	args := map[string]string{"location": "Shanghai"}
	argsRaw, _ := json.Marshal(args)

	var argsDoc any
	_ = json.Unmarshal(argsRaw, &argsDoc)

	toolUseId := "tool_123"
	toolName := "get_weather"

	inputTokens := int32(5)
	outputTokens := int32(15)
	totalTokens := int32(20)

	resp := &bedrockruntime.ConverseOutput{
		StopReason: types.StopReasonToolUse,
		Usage: &types.TokenUsage{
			InputTokens:  &inputTokens,
			OutputTokens: &outputTokens,
			TotalTokens:  &totalTokens,
		},
		Output: &types.ConverseOutputMemberMessage{
			Value: types.Message{
				Role: types.ConversationRoleAssistant,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberToolUse{
						Value: types.ToolUseBlock{
							ToolUseId: &toolUseId,
							Name:      &toolName,
							Input:     document.NewLazyDocument(argsDoc),
						},
					},
				},
			},
		},
	}

	result := convertResponse(resp)

	if result.FinishReason != "tool_use" {
		t.Errorf("FinishReason = %q，期望 tool_use", result.FinishReason)
	}
	if len(result.Message.ToolCalls) != 1 {
		t.Fatalf("ToolCalls = %d，期望 1", len(result.Message.ToolCalls))
	}
	tc := result.Message.ToolCalls[0]
	if tc.ID != "tool_123" {
		t.Errorf("ToolCall.ID = %q，期望 tool_123", tc.ID)
	}
	if tc.Function.Name != "get_weather" {
		t.Errorf("ToolCall.Name = %q，期望 get_weather", tc.Function.Name)
	}
	if tc.Function.Arguments == "" {
		t.Error("ToolCall.Arguments 不应为空")
	}
}

// ─── convertToolConfig ───────────────────────────────────────────────────────

func TestConvertToolConfig(t *testing.T) {
	params := json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}}}`)
	tools := []llm.Tool{
		{Function: llm.FunctionDef{
			Name:        "get_weather",
			Description: "获取天气信息",
			Parameters:  params,
		}},
	}

	cfg := convertToolConfig(tools)
	if cfg == nil {
		t.Fatal("convertToolConfig 返回 nil")
	}
	if len(cfg.Tools) != 1 {
		t.Fatalf("Tools = %d，期望 1", len(cfg.Tools))
	}

	toolMember, ok := cfg.Tools[0].(*types.ToolMemberToolSpec)
	if !ok {
		t.Fatal("Tool 不是 *types.ToolMemberToolSpec")
	}
	if *toolMember.Value.Name != "get_weather" {
		t.Errorf("Name = %q，期望 get_weather", *toolMember.Value.Name)
	}
	if *toolMember.Value.Description != "获取天气信息" {
		t.Errorf("Description = %q，期望 获取天气信息", *toolMember.Value.Description)
	}
	if toolMember.Value.InputSchema == nil {
		t.Error("InputSchema 不应为 nil")
	}
}
