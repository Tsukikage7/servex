package compose_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/Tsukikage7/servex/v2/llm/compose"
)

func TestGraph_Compile_Simple(t *testing.T) {
	g := compose.NewGraph[string, string]()
	require.NoError(t, g.AddLambdaNode("upper", compose.InvokableLambda(func(_ context.Context, s string) (string, error) {
		return s + "!", nil
	})))
	g.AddEdge(compose.START, "upper")
	g.AddEdge("upper", compose.END)

	r, err := g.Compile(context.Background())
	require.NoError(t, err)
	require.NotNil(t, r)
}

func TestGraph_Compile_MissingEdge(t *testing.T) {
	g := compose.NewGraph[string, string]()
	require.NoError(t, g.AddLambdaNode("upper", compose.InvokableLambda(func(_ context.Context, s string) (string, error) {
		return s, nil
	})))
	g.AddEdge("upper", compose.END)

	_, err := g.Compile(context.Background())
	assert.Error(t, err)
}

func TestGraph_Compile_Cycle(t *testing.T) {
	g := compose.NewGraph[string, string]()
	require.NoError(t, g.AddLambdaNode("a", compose.InvokableLambda(func(_ context.Context, s string) (string, error) { return s, nil })))
	require.NoError(t, g.AddLambdaNode("b", compose.InvokableLambda(func(_ context.Context, s string) (string, error) { return s, nil })))
	g.AddEdge(compose.START, "a")
	g.AddEdge("a", "b")
	g.AddEdge("b", "a")
	g.AddEdge("b", compose.END)

	_, err := g.Compile(context.Background())
	assert.Error(t, err)
}

func TestGraph_Compile_DuplicateNode(t *testing.T) {
	g := compose.NewGraph[string, string]()
	require.NoError(t, g.AddLambdaNode("step", compose.InvokableLambda(func(_ context.Context, s string) (string, error) { return s, nil })))
	err := g.AddLambdaNode("step", compose.InvokableLambda(func(_ context.Context, s string) (string, error) { return s, nil }))
	assert.Error(t, err)
}
