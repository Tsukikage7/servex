package agent_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/agent"
)

func TestBlackboard_SetGet(t *testing.T) {
	board := agent.NewBlackboard()
	board.Set("key", "value")

	v, ok := board.Get("key")
	assert.True(t, ok)
	assert.Equal(t, "value", v)

	_, ok = board.Get("missing")
	assert.False(t, ok)
}

func TestGetFromBoard_TypedAccess(t *testing.T) {
	board := agent.NewBlackboard()
	board.Set("count", 42)

	v, ok := agent.GetFromBoard[int](board, "count")
	assert.True(t, ok)
	assert.Equal(t, 42, v)

	_, ok = agent.GetFromBoard[string](board, "count")
	assert.False(t, ok)
}

func TestBlackboardAgent_WritesOutputToBoard(t *testing.T) {
	model := &fixedModel{reply: "analysis result"}
	a, _ := agent.New(&agent.Config{Name: "analyst", Model: model})
	board := agent.NewBlackboard()

	ba := agent.NewBlackboardAgent(a, board, "", "analysis")
	result, err := ba.Run(context.Background(), "analyze this")

	require.NoError(t, err)
	assert.Equal(t, "analysis result", result.Output)

	stored, ok := board.Get("analysis")
	assert.True(t, ok)
	assert.Equal(t, "analysis result", stored)
}

func TestBlackboardAgent_ReadsInputFromBoard(t *testing.T) {
	var capturedInput string
	captureModel := &captureInputModel{onInput: func(s string) { capturedInput = s }}
	a, _ := agent.New(&agent.Config{Name: "reader", Model: captureModel})

	board := agent.NewBlackboard()
	board.Set("context", "background info")

	ba := agent.NewBlackboardAgent(a, board, "context", "")
	_, err := ba.Run(context.Background(), "original input")
	require.NoError(t, err)

	assert.Contains(t, capturedInput, "background info")
	assert.Contains(t, capturedInput, "original input")
}

type captureInputModel struct {
	onInput func(string)
}

func (m *captureInputModel) Generate(_ context.Context, msgs []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
	for _, msg := range msgs {
		if msg.Role == llm.RoleUser {
			m.onInput(msg.Content)
		}
	}
	return &llm.ChatResponse{Message: llm.AssistantMessage("ok")}, nil
}

func (m *captureInputModel) Stream(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (llm.StreamReader, error) {
	return &singleTokenReader{token: "ok"}, nil
}
