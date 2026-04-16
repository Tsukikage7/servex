package compose_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tsukikage7/servex/v2/llm/compose"
)

// mockCallbackHandler 记录所有回调调用，用于测试验证.
type mockCallbackHandler struct {
	mu     sync.Mutex
	events []string
	nodes  []compose.NodeRunInfo
}

func (m *mockCallbackHandler) OnGraphStart(_ context.Context, _ compose.GraphRunInfo, _ any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "graph_start")
}

func (m *mockCallbackHandler) OnGraphEnd(_ context.Context, _ compose.GraphRunInfo, _ any, _ error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "graph_end")
}

func (m *mockCallbackHandler) OnNodeStart(_ context.Context, info compose.NodeRunInfo, _ any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "node_start:"+info.NodeID)
	m.nodes = append(m.nodes, info)
}

func (m *mockCallbackHandler) OnNodeEnd(_ context.Context, info compose.NodeRunInfo, _ any, _ error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "node_end:"+info.NodeID)
}

func (m *mockCallbackHandler) OnNodeSkip(_ context.Context, info compose.NodeRunInfo, _ string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, "node_skip:"+info.NodeID)
}

func TestRunnable_Callbacks(t *testing.T) {
	handler := &mockCallbackHandler{}

	g := compose.NewGraph[string, string]()
	require.NoError(t, g.AddLambdaNode("step1", compose.InvokableLambda(func(_ context.Context, s string) (string, error) {
		return s + "-1", nil
	})))
	require.NoError(t, g.AddLambdaNode("step2", compose.InvokableLambda(func(_ context.Context, s string) (string, error) {
		return s + "-2", nil
	})))
	g.AddEdge(compose.START, "step1")
	g.AddEdge("step1", "step2")
	g.AddEdge("step2", compose.END)

	r, err := g.Compile(context.Background(), compose.WithCallbacks(handler))
	require.NoError(t, err)

	out, err := r.Invoke(context.Background(), "x")
	require.NoError(t, err)
	assert.Equal(t, "x-1-2", out)

	handler.mu.Lock()
	defer handler.mu.Unlock()

	// 验证回调顺序
	assert.Equal(t, []string{
		"graph_start",
		"node_start:step1",
		"node_end:step1",
		"node_start:step2",
		"node_end:step2",
		"graph_end",
	}, handler.events)

	// 验证 NodeRunInfo 包含正确的 NodeID 和 NodeKind
	require.Len(t, handler.nodes, 2)
	assert.Equal(t, "step1", handler.nodes[0].NodeID)
	assert.Equal(t, "invoke", handler.nodes[0].NodeKind)
	assert.Equal(t, "step2", handler.nodes[1].NodeID)
	assert.Equal(t, "invoke", handler.nodes[1].NodeKind)

	// 验证 RunID 非空且两个节点共享同一 RunID
	assert.NotEmpty(t, handler.nodes[0].RunID)
	assert.Equal(t, handler.nodes[0].RunID, handler.nodes[1].RunID)
}

func TestRunnable_Callbacks_NodeSkip(t *testing.T) {
	handler := &mockCallbackHandler{}

	// START → check --branch--> [pathA, pathB]
	// pathA → END
	// pathB → END
	g := compose.NewGraph[int, string]()
	require.NoError(t, g.AddLambdaNode("check", compose.InvokableLambda(func(_ context.Context, n int) (int, error) {
		return n, nil
	})))
	require.NoError(t, g.AddLambdaNode("pathA", compose.InvokableLambda(func(_ context.Context, _ int) (string, error) {
		return "a", nil
	})))
	require.NoError(t, g.AddLambdaNode("pathB", compose.InvokableLambda(func(_ context.Context, _ int) (string, error) {
		return "b", nil
	})))

	g.AddEdge(compose.START, "check")
	g.AddBranch("check", compose.NewBranch(
		func(_ context.Context, n int) (string, error) {
			if n > 0 {
				return "pathA", nil
			}
			return "pathB", nil
		},
		"pathA", "pathB",
	))
	g.AddEdge("pathA", compose.END)
	g.AddEdge("pathB", compose.END)

	r, err := g.Compile(context.Background(), compose.WithCallbacks(handler))
	require.NoError(t, err)

	out, err := r.Invoke(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, "a", out)

	handler.mu.Lock()
	defer handler.mu.Unlock()

	// pathB 应被跳过
	assert.Contains(t, handler.events, "node_skip:pathB")
	// pathA 应正常执行
	assert.Contains(t, handler.events, "node_start:pathA")
	assert.Contains(t, handler.events, "node_end:pathA")
	// pathB 不应执行
	assert.NotContains(t, handler.events, "node_start:pathB")
}

func TestRunnable_Callbacks_NoCallbacks(t *testing.T) {
	// 无 callback 时不应 panic，走 NoopCallbackHandler 零开销路径
	g := compose.NewGraph[string, string]()
	require.NoError(t, g.AddLambdaNode("step", compose.InvokableLambda(func(_ context.Context, s string) (string, error) {
		return s + "!", nil
	})))
	g.AddEdge(compose.START, "step")
	g.AddEdge("step", compose.END)

	r, err := g.Compile(context.Background())
	require.NoError(t, err)

	out, err := r.Invoke(context.Background(), "hello")
	require.NoError(t, err)
	assert.Equal(t, "hello!", out)
}

func TestRunnable_Callbacks_MultipleHandlers(t *testing.T) {
	h1 := &mockCallbackHandler{}
	h2 := &mockCallbackHandler{}

	g := compose.NewGraph[string, string]()
	require.NoError(t, g.AddLambdaNode("step", compose.InvokableLambda(func(_ context.Context, s string) (string, error) {
		return s, nil
	})))
	g.AddEdge(compose.START, "step")
	g.AddEdge("step", compose.END)

	r, err := g.Compile(context.Background(), compose.WithCallbacks(h1, h2))
	require.NoError(t, err)

	_, err = r.Invoke(context.Background(), "x")
	require.NoError(t, err)

	// 两个 handler 都应收到回调
	assert.NotEmpty(t, h1.events)
	assert.NotEmpty(t, h2.events)
	assert.Equal(t, h1.events, h2.events)
}
