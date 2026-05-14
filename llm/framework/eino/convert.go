// Package eino adapts servex LLM facade types to CloudWeGo Eino.
package eino

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"

	"github.com/Tsukikage7/servex/v2/llm"
)

var (
	// ErrNilModel 表示传入的 Eino 模型为空.
	ErrNilModel = errors.New("llm/eino: model is nil")
	// ErrNilEmbedder 表示传入的 Eino embedding 组件为空.
	ErrNilEmbedder = errors.New("llm/eino: embedder is nil")
	// ErrUnsupportedMessage 表示消息无法无损转换为目标框架消息.
	ErrUnsupportedMessage = errors.New("llm/eino: unsupported message")
)

// ToEinoMessages 将 servex 消息列表转换为 Eino 消息列表.
func ToEinoMessages(messages []llm.Message) ([]*schema.Message, error) {
	out := make([]*schema.Message, 0, len(messages))
	for i := range messages {
		msg, err := ToEinoMessage(messages[i])
		if err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	return out, nil
}

// ToEinoMessage 将单条 servex 消息转换为 Eino 消息.
func ToEinoMessage(msg llm.Message) (*schema.Message, error) {
	switch msg.Role {
	case llm.RoleSystem:
		return schema.SystemMessage(msg.Content), nil
	case llm.RoleUser:
		out := schema.UserMessage(msg.Content)
		out.Name = msg.Name
		return out, nil
	case llm.RoleAssistant:
		out := schema.AssistantMessage(msg.Content, toEinoToolCalls(msg.ToolCalls))
		out.Name = msg.Name
		return out, nil
	case llm.RoleTool:
		out := schema.ToolMessage(msg.Content, msg.ToolCallID)
		out.Name = msg.Name
		return out, nil
	default:
		return nil, fmt.Errorf("%w: role %q", ErrUnsupportedMessage, msg.Role)
	}
}

// FromEinoMessage 将 Eino 消息转换为 servex 消息.
func FromEinoMessage(msg *schema.Message) llm.Message {
	if msg == nil {
		return llm.Message{}
	}
	out := llm.Message{
		Role:       fromEinoRole(msg.Role),
		Content:    msg.Content,
		Name:       msg.Name,
		ToolCallID: msg.ToolCallID,
		ToolCalls:  fromEinoToolCalls(msg.ToolCalls),
	}
	return out
}

// ToEinoTools 将 servex 工具定义转换为 Eino 工具定义.
func ToEinoTools(tools []llm.Tool) ([]*schema.ToolInfo, error) {
	out := make([]*schema.ToolInfo, 0, len(tools))
	for _, tool := range tools {
		converted, err := ToEinoTool(tool)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}

// ToEinoTool 将单个 servex 工具定义转换为 Eino 工具定义.
func ToEinoTool(tool llm.Tool) (*schema.ToolInfo, error) {
	info := &schema.ToolInfo{
		Name: tool.Function.Name,
		Desc: tool.Function.Description,
	}
	if len(tool.Function.Parameters) == 0 {
		return info, nil
	}
	var schemaDef jsonschema.Schema
	if err := json.Unmarshal(tool.Function.Parameters, &schemaDef); err != nil {
		return nil, fmt.Errorf("llm/eino: parse tool %q schema: %w", tool.Function.Name, err)
	}
	info.ParamsOneOf = schema.NewParamsOneOfByJSONSchema(&schemaDef)
	return info, nil
}

func toEinoToolCalls(calls []llm.ToolCall) []schema.ToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]schema.ToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, schema.ToolCall{
			ID: call.ID,
			Function: schema.FunctionCall{
				Name:      call.Function.Name,
				Arguments: call.Function.Arguments,
			},
		})
	}
	return out
}

func fromEinoToolCalls(calls []schema.ToolCall) []llm.ToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]llm.ToolCall, 0, len(calls))
	for _, call := range calls {
		tc := llm.ToolCall{ID: call.ID}
		tc.Function.Name = call.Function.Name
		tc.Function.Arguments = call.Function.Arguments
		out = append(out, tc)
	}
	return out
}

func fromEinoRole(role schema.RoleType) llm.Role {
	switch role {
	case schema.System:
		return llm.RoleSystem
	case schema.User:
		return llm.RoleUser
	case schema.Assistant:
		return llm.RoleAssistant
	case schema.Tool:
		return llm.RoleTool
	default:
		return llm.Role(role)
	}
}

func fromEinoUsage(usage *schema.TokenUsage) llm.Usage {
	if usage == nil {
		return llm.Usage{}
	}
	return llm.Usage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}
