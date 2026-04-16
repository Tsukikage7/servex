package agent_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/agent"
)

// mockAgentCallbackHandler 记录所有 Agent 回调调用.
type mockAgentCallbackHandler struct {
	mu          sync.Mutex
	events      []string
	llmCalls    int
	llmResps    int
	agentStarts int
	agentEnds   int
}

func (m *mockAgentCallbackHandler) OnAgentStart(_ context.Context, _ string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agentStarts++
	m.events = append(m.events, "agent_start")
}

func (m *mockAgentCallbackHandler) OnAgentEnd(_ context.Context, _ *agent.Result, _ error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agentEnds++
	m.events = append(m.events, "agent_end")
}

func (m *mockAgentCallbackHandler) OnLLMCall(_ context.Context, _ []llm.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.llmCalls++
	m.events = append(m.events, "llm_call")
}

func (m *mockAgentCallbackHandler) OnLLMResponse(_ context.Context, _ *llm.ChatResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.llmResps++
	m.events = append(m.events, "llm_response")
}

func (m *mockAgentCallbackHandler) OnToolCallStart(_ context.Context, _ llm.ToolCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "tool_call_start")
}

func (m *mockAgentCallbackHandler) OnToolCallEnd(_ context.Context, _ llm.ToolCall, _ string, _ error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "tool_call_end")
}

func TestAgent_Callbacks(t *testing.T) {
	handler := &mockAgentCallbackHandler{}
	model := &fixedModel{reply: "hello world"}

	a, err := agent.New(&agent.Config{
		Model:     model,
		Callbacks: []agent.AgentCallbackHandler{handler},
	})
	require.NoError(t, err)

	result, err := a.Run(context.Background(), "test input")
	require.NoError(t, err)
	assert.Equal(t, "hello world", result.Output)

	handler.mu.Lock()
	defer handler.mu.Unlock()

	// 验证回调顺序
	assert.Equal(t, []string{
		"agent_start",
		"llm_call",
		"llm_response",
		"agent_end",
	}, handler.events)

	assert.Equal(t, 1, handler.agentStarts)
	assert.Equal(t, 1, handler.agentEnds)
	assert.Equal(t, 1, handler.llmCalls)
	assert.Equal(t, 1, handler.llmResps)
}

func TestAgent_Callbacks_Stream(t *testing.T) {
	handler := &mockAgentCallbackHandler{}
	model := &fixedModel{reply: "streamed"}

	a, err := agent.New(&agent.Config{
		Model:     model,
		Callbacks: []agent.AgentCallbackHandler{handler},
	})
	require.NoError(t, err)

	ch, err := a.RunStream(context.Background(), "stream input")
	require.NoError(t, err)

	var output string
	for evt := range ch {
		if evt.Type == agent.EventOutput {
			output = evt.Content
		}
	}
	assert.Equal(t, "streamed", output)

	handler.mu.Lock()
	defer handler.mu.Unlock()

	assert.Equal(t, 1, handler.agentStarts)
	assert.Equal(t, 1, handler.agentEnds)
	assert.Contains(t, handler.events, "agent_start")
	assert.Contains(t, handler.events, "agent_end")
}

func TestAgent_Callbacks_NoCallbacks(t *testing.T) {
	// 无回调时不 panic
	model := &fixedModel{reply: "ok"}

	a, err := agent.New(&agent.Config{
		Model: model,
	})
	require.NoError(t, err)

	result, err := a.Run(context.Background(), "test")
	require.NoError(t, err)
	assert.Equal(t, "ok", result.Output)
}
