package compose_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/Tsukikage7/servex/v2/llm/compose"
)

func TestRunnable_Invoke_Linear(t *testing.T) {
	g := compose.NewGraph[string, string]()
	require.NoError(t, g.AddLambdaNode("upper", compose.InvokableLambda(func(_ context.Context, s string) (string, error) {
		return strings.ToUpper(s), nil
	})))
	require.NoError(t, g.AddLambdaNode("exclaim", compose.InvokableLambda(func(_ context.Context, s string) (string, error) {
		return s + "!", nil
	})))
	g.AddEdge(compose.START, "upper")
	g.AddEdge("upper", "exclaim")
	g.AddEdge("exclaim", compose.END)

	r, err := g.Compile(context.Background())
	require.NoError(t, err)

	out, err := r.Invoke(context.Background(), "hello")
	require.NoError(t, err)
	assert.Equal(t, "HELLO!", out)
}

func TestRunnable_Invoke_Branch(t *testing.T) {
	g := compose.NewGraph[int, string]()
	require.NoError(t, g.AddLambdaNode("check", compose.InvokableLambda(func(_ context.Context, n int) (int, error) {
		return n, nil
	})))
	require.NoError(t, g.AddLambdaNode("positive", compose.InvokableLambda(func(_ context.Context, n int) (string, error) {
		return "positive", nil
	})))
	require.NoError(t, g.AddLambdaNode("negative", compose.InvokableLambda(func(_ context.Context, n int) (string, error) {
		return "negative", nil
	})))
	g.AddEdge(compose.START, "check")
	g.AddBranch("check", compose.NewBranch(
		func(_ context.Context, n int) (string, error) {
			if n > 0 {
				return "positive", nil
			}
			return "negative", nil
		},
		"positive", "negative",
	))
	g.AddEdge("positive", compose.END)
	g.AddEdge("negative", compose.END)

	r, err := g.Compile(context.Background())
	require.NoError(t, err)

	out, err := r.Invoke(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, "positive", out)

	out, err = r.Invoke(context.Background(), -3)
	require.NoError(t, err)
	assert.Equal(t, "negative", out)
}

func TestRunnable_Invoke_WithState(t *testing.T) {
	type Counter struct{ Count int }

	g := compose.NewGraphWithState[string, string, *Counter](func() *Counter { return &Counter{} })
	require.NoError(t, g.AddLambdaNode("step1", compose.InvokableLambda(func(ctx context.Context, s string) (string, error) {
		state, _ := compose.GetState[*Counter](ctx)
		state.Count++
		return s + "-step1", nil
	})))
	require.NoError(t, g.AddLambdaNode("step2", compose.InvokableLambda(func(ctx context.Context, s string) (string, error) {
		state, _ := compose.GetState[*Counter](ctx)
		state.Count++
		return s + fmt.Sprintf("-count%d", state.Count), nil
	})))
	g.AddEdge(compose.START, "step1")
	g.AddEdge("step1", "step2")
	g.AddEdge("step2", compose.END)

	r, err := g.Compile(context.Background())
	require.NoError(t, err)

	out, err := r.Invoke(context.Background(), "x")
	require.NoError(t, err)
	assert.Equal(t, "x-step1-count2", out)
}

func TestRunnable_Invoke_ParallelFanout(t *testing.T) {
	// START → a → [b, c] → END（b 和 c 都直连 END，取最后完成的）
	g := compose.NewGraph[string, string]()
	require.NoError(t, g.AddLambdaNode("a", compose.InvokableLambda(func(_ context.Context, s string) (string, error) {
		return s + "-a", nil
	})))
	require.NoError(t, g.AddLambdaNode("b", compose.InvokableLambda(func(_ context.Context, s string) (string, error) {
		return s + "-b", nil
	})))
	require.NoError(t, g.AddLambdaNode("c", compose.InvokableLambda(func(_ context.Context, s string) (string, error) {
		return s + "-c", nil
	})))
	g.AddEdge(compose.START, "a")
	g.AddEdge("a", "b")
	g.AddEdge("a", "c")
	g.AddEdge("b", compose.END)
	g.AddEdge("c", compose.END)

	r, err := g.Compile(context.Background())
	require.NoError(t, err)

	out, err := r.Invoke(context.Background(), "x")
	require.NoError(t, err)
	// b 或 c 的结果（两者都是 x-a 为前缀）
	assert.True(t, strings.HasPrefix(out, "x-a-"))
}

func TestRunnable_Invoke_Branch_WithDownstream(t *testing.T) {
	// START → check --branch--> [pathA, pathB]
	// pathA → resultA → END
	// pathB → resultB → END
	// 选 pathA 时，resultB 也不应报错
	g := compose.NewGraph[int, string]()
	require.NoError(t, g.AddLambdaNode("check", compose.InvokableLambda(func(_ context.Context, n int) (int, error) {
		return n, nil
	})))
	require.NoError(t, g.AddLambdaNode("pathA", compose.InvokableLambda(func(_ context.Context, n int) (string, error) {
		return "a", nil
	})))
	require.NoError(t, g.AddLambdaNode("pathB", compose.InvokableLambda(func(_ context.Context, n int) (string, error) {
		return "b", nil
	})))
	require.NoError(t, g.AddLambdaNode("resultA", compose.InvokableLambda(func(_ context.Context, s string) (string, error) {
		return "result-" + s, nil
	})))
	require.NoError(t, g.AddLambdaNode("resultB", compose.InvokableLambda(func(_ context.Context, s string) (string, error) {
		return "result-" + s, nil
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
	g.AddEdge("pathA", "resultA")
	g.AddEdge("pathB", "resultB")
	g.AddEdge("resultA", compose.END)
	g.AddEdge("resultB", compose.END)

	r, err := g.Compile(context.Background())
	require.NoError(t, err)

	// 选 pathA：resultB 应被跳过，不报错
	out, err := r.Invoke(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, "result-a", out)

	// 选 pathB
	out, err = r.Invoke(context.Background(), -1)
	require.NoError(t, err)
	assert.Equal(t, "result-b", out)
}

func TestRunnable_Invoke_AutoConcatStreamNode(t *testing.T) {
	// Graph 中包含一个 StreamableLambda 节点
	// 调用 Invoke → 框架自动 concat 流输出为单值
	g := compose.NewGraph[string, string]()
	require.NoError(t, g.AddLambdaNode("stream_node", compose.WithConcatFunc(
		compose.StreamableLambda(func(_ context.Context, s string) (*compose.StreamReader[string], error) {
			return compose.NewSliceStreamReader([]string{s, "!", "!"}), nil
		}),
		func(a, b string) string { return a + b },
	)))
	g.AddEdge(compose.START, "stream_node")
	g.AddEdge("stream_node", compose.END)
	r, err := g.Compile(context.Background())
	require.NoError(t, err)

	out, err := r.Invoke(context.Background(), "hello")
	require.NoError(t, err)
	assert.Equal(t, "hello!!", out)
}

func TestRunnable_Stream_AutoBoxInvokeNode(t *testing.T) {
	// Graph 中包含一个 InvokableLambda 节点
	// 调用 Stream → 框架自动 box invoke 输出为流
	g := compose.NewGraph[string, string]()
	require.NoError(t, g.AddLambdaNode("invoke_node", compose.InvokableLambda(func(_ context.Context, s string) (string, error) {
		return s + "!", nil
	})))
	g.AddEdge(compose.START, "invoke_node")
	g.AddEdge("invoke_node", compose.END)
	r, err := g.Compile(context.Background())
	require.NoError(t, err)

	reader, err := r.Stream(context.Background(), "hello")
	require.NoError(t, err)
	v, err := reader.Recv()
	require.NoError(t, err)
	assert.Equal(t, "hello!", v)
}
