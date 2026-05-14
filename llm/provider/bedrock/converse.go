package bedrock

import (
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/Tsukikage7/servex/v2/llm"
)

// splitMessages 将 llm.Message 列表拆分为 Bedrock system blocks 和对话消息.
// Bedrock Converse API 中 system 消息通过 System 字段单独传递，不在 Messages 列表中.
func splitMessages(messages []llm.Message) ([]types.SystemContentBlock, []types.Message) {
	var sysBlocks []types.SystemContentBlock
	var convMsgs []types.Message

	for _, m := range messages {
		switch m.Role {
		case llm.RoleSystem:
			sysBlocks = append(sysBlocks, &types.SystemContentBlockMemberText{
				Value: m.Content,
			})
		case llm.RoleUser, llm.RoleAssistant:
			convMsgs = append(convMsgs, convertMessage(m))
		case llm.RoleTool:
			// tool result 消息映射为 user 角色，内容为 ToolResultBlock
			convMsgs = append(convMsgs, convertToolResultMessage(m))
		}
	}

	return sysBlocks, convMsgs
}

// convertMessages 将 llm.Message 切片转为 Bedrock types.Message 切片.
// 注意：system 消息需要调用方提前分离（通过 splitMessages）.
func convertMessages(messages []llm.Message) []types.Message {
	_, convMsgs := splitMessages(messages)
	return convMsgs
}

// convertMessage 将单个 llm.Message 转为 Bedrock types.Message.
func convertMessage(m llm.Message) types.Message {
	var role types.ConversationRole
	switch m.Role {
	case llm.RoleAssistant:
		role = types.ConversationRoleAssistant
	default:
		role = types.ConversationRoleUser
	}

	var content []types.ContentBlock

	// 工具调用（assistant 消息中的 tool_use blocks）
	if len(m.ToolCalls) > 0 {
		if m.Content != "" {
			content = append(content, &types.ContentBlockMemberText{Value: m.Content})
		}
		for _, tc := range m.ToolCalls {
			// 将 JSON 参数字符串转为 document.Interface
			var inputVal any
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &inputVal); err != nil {
					// 兜底：用原始字符串作为 document
					inputVal = map[string]any{"raw": tc.Function.Arguments}
				}
			}
			name := tc.Function.Name
			id := tc.ID
			content = append(content, &types.ContentBlockMemberToolUse{
				Value: types.ToolUseBlock{
					ToolUseId: &id,
					Name:      &name,
					Input:     document.NewLazyDocument(inputVal),
				},
			})
		}
		return types.Message{Role: role, Content: content}
	}

	// 多模态内容
	if len(m.Parts) > 0 {
		for _, p := range m.Parts {
			switch p.Type {
			case llm.ContentTypeText:
				content = append(content, &types.ContentBlockMemberText{Value: p.Text})
				// image 暂不支持（需要 base64 解码），跳过
			}
		}
	} else if m.Content != "" {
		content = append(content, &types.ContentBlockMemberText{Value: m.Content})
	}

	return types.Message{Role: role, Content: content}
}

// convertToolResultMessage 将 tool 角色消息转换为 Bedrock user 消息（ToolResultBlock）.
func convertToolResultMessage(m llm.Message) types.Message {
	id := m.ToolCallID
	block := &types.ContentBlockMemberToolResult{
		Value: types.ToolResultBlock{
			ToolUseId: &id,
			Content: []types.ToolResultContentBlock{
				&types.ToolResultContentBlockMemberText{Value: m.Content},
			},
		},
	}
	return types.Message{
		Role:    types.ConversationRoleUser,
		Content: []types.ContentBlock{block},
	}
}

// convertToolConfig 将 llm.Tool 切片转为 Bedrock ToolConfiguration.
func convertToolConfig(tools []llm.Tool) *types.ToolConfiguration {
	bedTools := make([]types.Tool, 0, len(tools))
	for _, t := range tools {
		name := t.Function.Name
		desc := t.Function.Description

		// Parameters 是 JSON Schema（json.RawMessage），需要转为 document.Interface
		var schemaVal any
		if len(t.Function.Parameters) > 0 {
			if err := json.Unmarshal(t.Function.Parameters, &schemaVal); err != nil {
				// 兜底：用原始字符串作为 document
				schemaVal = map[string]any{"raw": string(t.Function.Parameters)}
			}
		}

		spec := types.ToolSpecification{
			Name:        &name,
			Description: &desc,
			InputSchema: &types.ToolInputSchemaMemberJson{
				Value: document.NewLazyDocument(schemaVal),
			},
		}
		bedTools = append(bedTools, &types.ToolMemberToolSpec{Value: spec})
	}
	return &types.ToolConfiguration{Tools: bedTools}
}

// convertResponse 将 Bedrock ConverseOutput 转为 llm.ChatResponse.
func convertResponse(resp *bedrockruntime.ConverseOutput) *llm.ChatResponse {
	result := &llm.ChatResponse{
		FinishReason: string(resp.StopReason),
	}

	if resp.Usage != nil {
		if resp.Usage.InputTokens != nil {
			result.Usage.PromptTokens = int(*resp.Usage.InputTokens)
		}
		if resp.Usage.OutputTokens != nil {
			result.Usage.CompletionTokens = int(*resp.Usage.OutputTokens)
		}
		if resp.Usage.TotalTokens != nil {
			result.Usage.TotalTokens = int(*resp.Usage.TotalTokens)
		}
	}

	// 提取消息
	if out, ok := resp.Output.(*types.ConverseOutputMemberMessage); ok {
		result.Message = extractMessage(out.Value)
	}

	return result
}

// extractMessage 将 Bedrock types.Message 转为 llm.Message.
func extractMessage(m types.Message) llm.Message {
	msg := llm.Message{Role: llm.RoleAssistant}

	for _, block := range m.Content {
		switch b := block.(type) {
		case *types.ContentBlockMemberText:
			msg.Content += b.Value
		case *types.ContentBlockMemberToolUse:
			tc := llm.ToolCall{ID: ""}
			if b.Value.ToolUseId != nil {
				tc.ID = *b.Value.ToolUseId
			}
			if b.Value.Name != nil {
				tc.Function.Name = *b.Value.Name
			}
			// 将 document.Interface 序列化回 JSON 字符串
			if b.Value.Input != nil {
				if raw, err := json.Marshal(b.Value.Input); err == nil {
					tc.Function.Arguments = string(raw)
				}
			}
			msg.ToolCalls = append(msg.ToolCalls, tc)
		}
	}

	return msg
}
