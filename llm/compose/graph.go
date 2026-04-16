package compose

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm"
)

// nodeEntry 图中一个节点条目.
type nodeEntry struct {
	id string
	nf nodeFunc
}

// Graph[I,O] 有向无环图编排构建器.
type Graph[I, O any] struct {
	nodes    []nodeEntry
	nodeIDs  map[string]struct{}
	edges    []edge
	branches []branchEntry
}

// NewGraph 创建无状态 Graph.
func NewGraph[I, O any]() *Graph[I, O] {
	return &Graph[I, O]{nodeIDs: make(map[string]struct{})}
}

// AddLambdaNode 注册 Lambda 节点，返回重复 ID 时的错误.
func (g *Graph[I, O]) AddLambdaNode(id string, nf nodeFunc) error {
	if err := g.checkID(id); err != nil {
		return err
	}
	if err := validateNodeFunc(nf); err != nil {
		return fmt.Errorf("compose: node %q: %w", id, err)
	}
	g.nodes = append(g.nodes, nodeEntry{id: id, nf: nf})
	g.nodeIDs[id] = struct{}{}
	return nil
}

// AddChatModelNode 注册 ChatModel 节点（[]llm.Message → StreamReader[llm.StreamChunk]）.
func (g *Graph[I, O]) AddChatModelNode(id string, model llm.ChatModel) error {
	if err := g.checkID(id); err != nil {
		return err
	}
	nf := StreamableLambda(func(ctx context.Context, msgs []llm.Message) (*StreamReader[llm.StreamChunk], error) {
		reader, err := model.Stream(ctx, msgs)
		if err != nil {
			return nil, err
		}
		ch := make(chan llm.StreamChunk, 16)
		go func() {
			defer close(ch)
			defer reader.Close()
			for {
				chunk, recvErr := reader.Recv()
				if recvErr != nil {
					break
				}
				select {
				case ch <- chunk:
				case <-ctx.Done():
					return
				}
			}
		}()
		return NewChanStreamReader(ch), nil
	})
	g.nodes = append(g.nodes, nodeEntry{id: id, nf: nf})
	g.nodeIDs[id] = struct{}{}
	return nil
}

// AddEdge 添加普通有向边.
func (g *Graph[I, O]) AddEdge(from, to string) {
	g.edges = append(g.edges, edge{from: from, to: to})
}

// AddBranch 从 from 节点添加条件边.
func (g *Graph[I, O]) AddBranch(from string, branch *Branch) {
	g.branches = append(g.branches, branchEntry{from: from, branch: branch})
}

func (g *Graph[I, O]) checkID(id string) error {
	if id == START || id == END {
		return fmt.Errorf("compose: node ID %q is reserved", id)
	}
	if _, exists := g.nodeIDs[id]; exists {
		return fmt.Errorf("compose: duplicate node ID %q", id)
	}
	return nil
}

// Compile 编译图，返回 Runnable.
func (g *Graph[I, O]) Compile(ctx context.Context, opts ...CompileOption) (*Runnable[I, O], error) {
	var co compileOptions
	for _, opt := range opts {
		opt(&co)
	}
	return compileGraph[I, O](ctx, g, nil, co.callbacks)
}

// GraphWithState[I,O,S] 带共享 State 的图.
type GraphWithState[I, O, S any] struct {
	graph   *Graph[I, O]
	factory StateFactory[S]
}

// NewGraphWithState 创建带 State 的图.
func NewGraphWithState[I, O, S any](factory StateFactory[S]) *GraphWithState[I, O, S] {
	return &GraphWithState[I, O, S]{graph: NewGraph[I, O](), factory: factory}
}

// AddLambdaNode 代理到内部 Graph.
func (g *GraphWithState[I, O, S]) AddLambdaNode(id string, nf nodeFunc) error {
	return g.graph.AddLambdaNode(id, nf)
}

// AddChatModelNode 代理到内部 Graph.
func (g *GraphWithState[I, O, S]) AddChatModelNode(id string, model llm.ChatModel) error {
	return g.graph.AddChatModelNode(id, model)
}

// AddEdge 代理到内部 Graph.
func (g *GraphWithState[I, O, S]) AddEdge(from, to string) {
	g.graph.AddEdge(from, to)
}

// AddBranch 代理到内部 Graph.
func (g *GraphWithState[I, O, S]) AddBranch(from string, branch *Branch) {
	g.graph.AddBranch(from, branch)
}

// Compile 编译带 State 的图.
func (g *GraphWithState[I, O, S]) Compile(ctx context.Context, opts ...CompileOption) (*Runnable[I, O], error) {
	var co compileOptions
	for _, opt := range opts {
		opt(&co)
	}
	injector := func(ctx context.Context) context.Context {
		return injectState(ctx, g.factory())
	}
	return compileGraph[I, O](ctx, g.graph, injector, co.callbacks)
}
