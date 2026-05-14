package adk

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/genai"

	"github.com/Tsukikage7/servex/v2/llm"
)

func TestNewAgentWrapsADKAgent(t *testing.T) {
	t.Parallel()

	wrapped, err := NewAgent(Config{
		Name:        "assistant",
		Description: "servex wrapper",
	})
	if err != nil {
		t.Fatalf("NewAgent failed: %v", err)
	}
	if wrapped.Name() != "assistant" {
		t.Fatalf("name mismatch: %q", wrapped.Name())
	}
	if wrapped.Agent() == nil {
		t.Fatal("expected underlying ADK agent")
	}
}

func TestWrapAgentRejectsNil(t *testing.T) {
	t.Parallel()

	if _, err := WrapAgent(nil); err == nil {
		t.Fatal("expected error for nil ADK agent")
	}
}

func TestWrapAgentPreservesUnderlyingAgent(t *testing.T) {
	t.Parallel()

	raw, err := agent.New(agent.Config{
		Name:        "raw",
		Description: "raw agent",
	})
	if err != nil {
		t.Fatalf("adk agent creation failed: %v", err)
	}

	wrapped, err := WrapAgent(raw)
	if err != nil {
		t.Fatalf("WrapAgent failed: %v", err)
	}
	if wrapped.Agent() != raw {
		t.Fatal("underlying agent was not preserved")
	}
	if wrapped.Description() != "raw agent" {
		t.Fatalf("description mismatch: %q", wrapped.Description())
	}
}

type stubServexChatModel struct {
	messages []llm.Message
	opts     llm.CallOptions
	resp     *llm.ChatResponse
}

func (m *stubServexChatModel) Generate(_ context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	m.messages = messages
	m.opts = llm.ApplyOptions(opts)
	return m.resp, nil
}

func (m *stubServexChatModel) Stream(context.Context, []llm.Message, ...llm.CallOption) (llm.StreamReader, error) {
	panic("not used")
}

func TestAsModelAdaptsServexChatModelToADKLLM(t *testing.T) {
	t.Parallel()

	base := &stubServexChatModel{resp: &llm.ChatResponse{
		Message:      llm.AssistantMessage("ok"),
		FinishReason: "stop",
		Usage:        llm.Usage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5},
		ModelID:      "actual",
	}}
	chatModel, err := AsModel("servex", base)
	if err != nil {
		t.Fatalf("AsModel failed: %v", err)
	}

	temp := float32(0.3)
	maxTokens := int32(128)
	req := &model.LLMRequest{
		Model: "configured",
		Contents: []*genai.Content{
			genai.NewContentFromText("hello", genai.RoleUser),
		},
		Config: &genai.GenerateContentConfig{
			Temperature:     &temp,
			MaxOutputTokens: maxTokens,
		},
	}

	var got *model.LLMResponse
	for resp, err := range chatModel.GenerateContent(t.Context(), req, false) {
		if err != nil {
			t.Fatalf("GenerateContent failed: %v", err)
		}
		got = resp
	}
	if got == nil || got.Content == nil || len(got.Content.Parts) != 1 || got.Content.Parts[0].Text != "ok" {
		t.Fatalf("ADK response mismatch: %+v", got)
	}
	if got.UsageMetadata == nil || got.UsageMetadata.TotalTokenCount != 5 {
		t.Fatalf("usage mismatch: %+v", got.UsageMetadata)
	}
	if len(base.messages) != 1 || base.messages[0].Content != "hello" {
		t.Fatalf("servex messages mismatch: %+v", base.messages)
	}
	if base.opts.Model != "configured" || base.opts.Temperature == nil || math.Abs(*base.opts.Temperature-0.3) > 0.000001 {
		t.Fatalf("servex options mismatch: %+v", base.opts)
	}
	if base.opts.MaxTokens == nil || *base.opts.MaxTokens != 128 {
		t.Fatalf("max tokens option mismatch: %+v", base.opts)
	}
}

func TestFromADKContentsPreservesAllTextParts(t *testing.T) {
	t.Parallel()

	messages, err := fromADKContents([]*genai.Content{{
		Role: string(genai.RoleUser),
		Parts: []*genai.Part{
			{Text: "hello"},
			{Text: " world"},
		},
	}})
	if err != nil {
		t.Fatalf("fromADKContents 失败: %v", err)
	}
	if len(messages) != 1 || messages[0].Content != "hello world" {
		t.Fatalf("文本片段未完整保留: %+v", messages)
	}
}

func TestToADKResponsePreservesToolCalls(t *testing.T) {
	t.Parallel()

	resp, err := toADKResponse(&llm.ChatResponse{
		Message: llm.Message{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{{
				ID: "call_1",
				Function: struct {
					Name      string
					Arguments string
				}{Name: "search", Arguments: `{"q":"go"}`},
			}},
		},
		FinishReason: "tool_calls",
	}, false)
	if err != nil {
		t.Fatalf("toADKResponse 失败: %v", err)
	}
	if resp.Content == nil || len(resp.Content.Parts) != 1 || resp.Content.Parts[0].FunctionCall == nil {
		t.Fatalf("工具调用未转换为 ADK function call: %+v", resp.Content)
	}
	call := resp.Content.Parts[0].FunctionCall
	if call.ID != "call_1" || call.Name != "search" {
		t.Fatalf("工具调用元数据不匹配: %+v", call)
	}
	gotArgs, err := json.Marshal(call.Args)
	if err != nil {
		t.Fatalf("工具参数无法 marshal: %v", err)
	}
	if string(gotArgs) != `{"q":"go"}` {
		t.Fatalf("工具参数不匹配: %s", gotArgs)
	}
}

func TestNewLLMAgentUsesServexModel(t *testing.T) {
	t.Parallel()

	base := &stubServexChatModel{resp: &llm.ChatResponse{Message: llm.AssistantMessage("ok")}}
	wrapped, err := NewLLMAgent(LLMAgentConfig{
		Name:        "assistant",
		Description: "uses servex model",
		Instruction: "be concise",
		Model:       base,
	})
	if err != nil {
		t.Fatalf("NewLLMAgent failed: %v", err)
	}
	if wrapped.Name() != "assistant" || wrapped.Agent() == nil {
		t.Fatalf("agent mismatch: %+v", wrapped)
	}
}
