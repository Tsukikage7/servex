# llm/compose

`github.com/Tsukikage7/servex/v2/llm/compose` — DAG 编排引擎，对标 Eino compose，支持四范式节点、条件边与共享 State。

## 功能特性

- 泛型 `Graph[I,O]` 构建有向无环图，编译为类型安全的 `Runnable[I,O]`
- 四范式节点：Invoke（同步）、Stream（流式输出）、Collect（流式输入）、Transform（双向流）
- 条件边 `Branch`：根据节点输出动态选择下游节点
- 共享 `State`：通过 Context 在节点间传递可变状态
- 内置 `AddChatModelNode`：快速注册 `llm.ChatModel` 节点

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/llm/compose
```

## 核心类型

```go
// 图构建器
type Graph[I, O any] struct { ... }
func NewGraph[I, O any]() *Graph[I, O]

// 带共享 State 的图
type GraphWithState[I, O, S any] struct { ... }
func NewGraphWithState[I, O, S any](factory StateFactory[S]) *GraphWithState[I, O, S]

// 编译后的可执行图
type Runnable[I, O any] struct { ... }
func (r *Runnable[I, O]) Invoke(ctx context.Context, input I) (O, error)
func (r *Runnable[I, O]) Stream(ctx context.Context, input I) (*StreamReader[O], error)

// 条件边
type Branch struct { ... }
func NewBranch[T any](condFn func(context.Context, T) (string, error), targets ...string) *Branch

// 虚拟节点 key
const START = "__start__"
const END   = "__end__"
```

## 四范式节点构造函数

```go
// Invoke：同步输入 → 同步输出
InvokableLambda[I, O any](fn func(context.Context, I) (O, error)) nodeFunc

// Stream：同步输入 → 流式输出
StreamableLambda[I, O any](fn func(context.Context, I) (*StreamReader[O], error)) nodeFunc

// Collect：流式输入 → 同步输出
CollectableLambda[I, O any](fn func(context.Context, *StreamReader[I]) (O, error)) nodeFunc

// Transform：流式输入 → 流式输出
TransformableLambda[I, O any](fn func(context.Context, *StreamReader[I]) (*StreamReader[O], error)) nodeFunc
```

## 使用示例

### 线性图

```go
import "github.com/Tsukikage7/servex/v2/llm/compose"

g := compose.NewGraph[string, string]()

_ = g.AddLambdaNode("upper", compose.InvokableLambda(
    func(_ context.Context, s string) (string, error) {
        return strings.ToUpper(s), nil
    },
))
_ = g.AddLambdaNode("exclaim", compose.InvokableLambda(
    func(_ context.Context, s string) (string, error) {
        return s + "!", nil
    },
))

g.AddEdge(compose.START, "upper")
g.AddEdge("upper", "exclaim")
g.AddEdge("exclaim", compose.END)

r, err := g.Compile(context.Background())
if err != nil {
    log.Fatal(err)
}

out, _ := r.Invoke(context.Background(), "hello")
fmt.Println(out) // HELLO!
```

### 条件分支图

```go
g := compose.NewGraph[string, string]()

_ = g.AddLambdaNode("classify", compose.InvokableLambda(
    func(_ context.Context, s string) (string, error) { return s, nil },
))
_ = g.AddLambdaNode("greet", compose.InvokableLambda(
    func(_ context.Context, s string) (string, error) { return "你好，" + s, nil },
))
_ = g.AddLambdaNode("farewell", compose.InvokableLambda(
    func(_ context.Context, s string) (string, error) { return "再见，" + s, nil },
))

branch := compose.NewBranch(
    func(_ context.Context, s string) (string, error) {
        if strings.HasPrefix(s, "hello") {
            return "greet", nil
        }
        return "farewell", nil
    },
    "greet", "farewell",
)

g.AddEdge(compose.START, "classify")
g.AddBranch("classify", branch)
g.AddEdge("greet", compose.END)
g.AddEdge("farewell", compose.END)

r, _ := g.Compile(context.Background())
out, _ := r.Invoke(context.Background(), "hello world")
fmt.Println(out) // 你好，hello world
```

### 带共享 State 的图

```go
type MyState struct {
    Count int
}

g := compose.NewGraphWithState[string, string, *MyState](func() *MyState {
    return &MyState{}
})

_ = g.AddLambdaNode("counter", compose.InvokableLambda(
    func(ctx context.Context, s string) (string, error) {
        state, _ := compose.GetState[*MyState](ctx)
        state.Count++
        return fmt.Sprintf("%s (count=%d)", s, state.Count), nil
    },
))

g.AddEdge(compose.START, "counter")
g.AddEdge("counter", compose.END)

r, _ := g.Compile(context.Background())
out, _ := r.Invoke(context.Background(), "test")
fmt.Println(out) // test (count=1)
```

## 许可证

详见项目根目录 LICENSE 文件。
