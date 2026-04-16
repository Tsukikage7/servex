package agent_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/Tsukikage7/servex/v2/llm/agent"
)

func TestFanOut_RunsAllAgentsConcurrently(t *testing.T) {
	newAgent := func(reply string) *agent.Agent {
		a, _ := agent.New(&agent.Config{
			Name:  reply,
			Model: &fixedModel{reply: reply},
		})
		return a
	}

	fanout := agent.NewFanOut(map[string]*agent.Agent{
		"a": newAgent("reply-a"),
		"b": newAgent("reply-b"),
		"c": newAgent("reply-c"),
	})

	start := time.Now()
	results, err := fanout.Run(context.Background(), "hello")
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Len(t, results, 3)
	assert.Equal(t, "reply-a", results["a"].Output)
	assert.Equal(t, "reply-b", results["b"].Output)
	assert.Equal(t, "reply-c", results["c"].Output)
	assert.Less(t, elapsed, 500*time.Millisecond)
}

func TestFanOut_ReturnsErrorOnFailure(t *testing.T) {
	failAgent, _ := agent.New(&agent.Config{Name: "fail", Model: &errorModel{err: errors.New("boom")}})
	okAgent, _ := agent.New(&agent.Config{Name: "ok", Model: &fixedModel{reply: "ok"}})

	fanout := agent.NewFanOut(map[string]*agent.Agent{
		"fail": failAgent,
		"ok":   okAgent,
	})

	_, err := fanout.Run(context.Background(), "hello")
	assert.Error(t, err)
}
