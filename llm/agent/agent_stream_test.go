package agent_test

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/agent"
	"github.com/Tsukikage7/servex/v2/llm/agent/toolcall"
)

// mockStreamModel 返回固定 token 序列的 mock ChatModel.
type mockStreamModel struct {
	tokens []string
}

func (m *mockStreamModel) Generate(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
	content := ""
	for _, t := range m.tokens {
		content += t
	}
	return &llm.ChatResponse{Message: llm.AssistantMessage(content)}, nil
}

func (m *mockStreamModel) Stream(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (llm.StreamReader, error) {
	return &mockReader{tokens: m.tokens, pos: 0}, nil
}

type mockReader struct {
	tokens []string
	pos    int
	full   string
}

func (r *mockReader) Recv() (llm.StreamChunk, error) {
	if r.pos >= len(r.tokens) {
		return llm.StreamChunk{}, io.EOF
	}
	t := r.tokens[r.pos]
	r.full += t
	r.pos++
	return llm.StreamChunk{Delta: t}, nil
}

func (r *mockReader) Response() *llm.ChatResponse {
	return &llm.ChatResponse{Message: llm.AssistantMessage(r.full)}
}

func (r *mockReader) Close() error { return nil }

func TestRunStream_EmitsEventToken(t *testing.T) {
	model := &mockStreamModel{tokens: []string{"Hello", ", ", "world", "!"}}
	a, err := agent.New(&agent.Config{
		Name:  "test",
		Model: model,
	})
	require.NoError(t, err)

	ch, err := a.RunStream(context.Background(), "hi")
	require.NoError(t, err)

	var tokens []string
	var gotOutput bool
	for evt := range ch {
		switch evt.Type {
		case agent.EventToken:
			tokens = append(tokens, evt.Content)
		case agent.EventOutput:
			gotOutput = true
			assert.Equal(t, "Hello, world!", evt.Content)
		case agent.EventError:
			t.Fatalf("unexpected error event: %s", evt.Content)
		}
	}

	assert.Equal(t, []string{"Hello", ", ", "world", "!"}, tokens)
	assert.True(t, gotOutput)
}

// mockStreamModelWithTools 第一轮返回 tool_call（流式），第二轮返回最终文本（流式）.
type mockStreamModelWithTools struct {
	round int
}

func (m *mockStreamModelWithTools) Generate(_ context.Context, msgs []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	m.round++
	// 执行 StreamCallback（如果有）
	co := llm.ApplyOptions(opts)
	if m.round == 1 {
		// 第一轮：tool call，无 delta
		tc := llm.ToolCall{ID: "c1"}
		tc.Function.Name = "calc"
		tc.Function.Arguments = `{"x":1}`
		if co.StreamFunc != nil {
			_ = co.StreamFunc(context.Background(), llm.StreamChunk{
				ToolCalls:    []llm.ToolCall{tc},
				FinishReason: "tool_calls",
			})
		}
		return &llm.ChatResponse{
			Message: llm.Message{
				Role:      llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{tc},
			},
			FinishReason: "tool_calls",
		}, nil
	}
	// 第二轮：最终文本，逐 token 回调
	tokens := []string{"result", " is", " 42"}
	for _, t := range tokens {
		if co.StreamFunc != nil {
			_ = co.StreamFunc(context.Background(), llm.StreamChunk{Delta: t})
		}
	}
	return &llm.ChatResponse{Message: llm.AssistantMessage("result is 42")}, nil
}

func (m *mockStreamModelWithTools) Stream(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (llm.StreamReader, error) {
	return &mockReader{tokens: []string{"result is 42"}}, nil
}

func TestRunStream_WithTools_EmitsTokenAndToolEvents(t *testing.T) {
	reg := toolcall.NewRegistry()
	reg.Register(llm.Tool{
		Function: llm.FunctionDef{
			Name:       "calc",
			Parameters: json.RawMessage(`{"type":"object","properties":{"x":{"type":"number"}}}`),
		},
	}, func(_ context.Context, _ string) (string, error) {
		return `{"result":42}`, nil
	})

	model := &mockStreamModelWithTools{}
	a, err := agent.New(&agent.Config{
		Name:  "test",
		Model: model,
		Tools: reg,
	})
	require.NoError(t, err)

	ch, err := a.RunStream(context.Background(), "calc 1")
	require.NoError(t, err)

	var tokens []string
	var toolCallSeen, toolResultSeen, outputSeen bool
	for evt := range ch {
		switch evt.Type {
		case agent.EventToken:
			tokens = append(tokens, evt.Content)
		case agent.EventToolCall:
			toolCallSeen = true
			assert.Equal(t, "calc", evt.ToolCall.Function.Name)
		case agent.EventToolResult:
			toolResultSeen = true
		case agent.EventOutput:
			outputSeen = true
			assert.Equal(t, "result is 42", evt.Content)
		case agent.EventError:
			t.Fatalf("unexpected error: %s", evt.Content)
		}
	}

	assert.True(t, toolCallSeen)
	assert.True(t, toolResultSeen)
	assert.True(t, outputSeen)
	assert.Equal(t, []string{"result", " is", " 42"}, tokens)
}
