package compose

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Runnable[I,O] 编译后的可执行图.
type Runnable[I, O any] struct {
	order         []string
	nodes         map[string]nodeEntry
	successors    map[string][]string
	predecessors  map[string][]string
	branches      map[string]*Branch
	stateInjector func(context.Context) context.Context
	callback      CallbackHandler
}

// Invoke 同步调用.
func (r *Runnable[I, O]) Invoke(ctx context.Context, input I) (O, error) {
	if r.stateInjector != nil {
		ctx = r.stateInjector(ctx)
	}
	outputs, err := r.run(ctx, input, modeInvoke)
	if err != nil {
		var zero O
		return zero, err
	}
	return castOutput[O](outputs[END])
}

// Stream 流式调用，返回 *StreamReader[O].
func (r *Runnable[I, O]) Stream(ctx context.Context, input I) (*StreamReader[O], error) {
	if r.stateInjector != nil {
		ctx = r.stateInjector(ctx)
	}
	outputs, err := r.run(ctx, input, modeStreaming)
	if err != nil {
		return nil, err
	}
	raw := outputs[END]
	if sr, ok := raw.(*StreamReader[O]); ok {
		return sr, nil
	}
	v, err := castOutput[O](raw)
	if err != nil {
		return nil, err
	}
	return BoxStream(v), nil
}

// Collect 收集流式输入，返回同步输出.
func (r *Runnable[I, O]) Collect(ctx context.Context, inputReader *StreamReader[I], concat func(I, I) I) (O, error) {
	input, err := ConcatStream(inputReader, concat)
	if err != nil {
		var zero O
		return zero, err
	}
	return r.Invoke(ctx, input)
}

// Transform 流式输入，流式输出.
func (r *Runnable[I, O]) Transform(ctx context.Context, inputReader *StreamReader[I], concat func(I, I) I) (*StreamReader[O], error) {
	input, err := ConcatStream(inputReader, concat)
	if err != nil {
		return nil, err
	}
	return r.Stream(ctx, input)
}

// run 核心执行引擎：按拓扑顺序调度节点，支持条件边跳过.
func (r *Runnable[I, O]) run(ctx context.Context, input I, mode callMode) (map[string]any, error) {
	runID := uuid.New().String()
	graphInfo := GraphRunInfo{RunID: runID}

	r.callback.OnGraphStart(ctx, graphInfo, input)

	outputs := make(map[string]any, len(r.nodes)+2)
	outputs[START] = input

	// 记录被跳过的节点（条件边未选中）
	skipped := make(map[string]bool)

	// branch 覆盖输入（独立于 outputs，避免 key 命名冲突）
	branchInputs := make(map[string]any)

	for _, nodeID := range r.order {
		entry, ok := r.nodes[nodeID]
		if !ok {
			err := &NodeError{NodeID: nodeID, Err: fmt.Errorf("node not registered")}
			r.callback.OnGraphEnd(ctx, graphInfo, nil, err)
			return nil, err
		}

		nodeInfo := NodeRunInfo{
			NodeID:       nodeID,
			NodeKind:     nodeKindString(entry.nf.kind),
			GraphRunInfo: graphInfo,
		}

		// 如果该节点被条件边跳过，则跳过执行
		if skipped[nodeID] {
			r.callback.OnNodeSkip(ctx, nodeInfo, "branch not selected")
			continue
		}

		// 取第一个有输出的前驱作为输入
		// 优先检查 branch 覆盖输入键
		var nodeInput any
		if override, ok := branchInputs[nodeID]; ok {
			nodeInput = override
		} else {
			preds := r.predecessors[nodeID]
			for _, pred := range preds {
				if skipped[pred] {
					continue
				}
				if v, ok := outputs[pred]; ok {
					nodeInput = v
					break
				}
			}
		}

		if nodeInput == nil {
			err := &NodeError{NodeID: nodeID, Err: fmt.Errorf("no input available from predecessors")}
			r.callback.OnGraphEnd(ctx, graphInfo, nil, err)
			return nil, err
		}

		r.callback.OnNodeStart(ctx, nodeInfo, nodeInput)
		output, err := adaptedExecNode(ctx, entry.nf, nodeInput, mode)
		r.callback.OnNodeEnd(ctx, nodeInfo, output, err)
		if err != nil {
			nodeErr := &NodeError{NodeID: nodeID, Err: err}
			r.callback.OnGraphEnd(ctx, graphInfo, nil, nodeErr)
			return nil, nodeErr
		}
		outputs[nodeID] = output

		// 处理条件边：评估 branch，标记未选中的目标节点及其后代为跳过
		if branch, hasBranch := r.branches[nodeID]; hasBranch {
			target, branchErr := branch.Evaluate(ctx, output)
			if branchErr != nil {
				nodeErr := &NodeError{NodeID: nodeID, Err: branchErr}
				r.callback.OnGraphEnd(ctx, graphInfo, nil, nodeErr)
				return nil, nodeErr
			}
			// 若 branch 直达 END，立即设置输出并返回
			if target == END {
				outputs[END] = output
				r.callback.OnGraphEnd(ctx, graphInfo, output, nil)
				return outputs, nil
			}
			// 设置覆盖输入（branch 目标节点的 predecessors 可能包含非 branch 来源节点）
			branchInputs[target] = output
			// BFS 传播跳过：标记所有未选中分支的后代节点
			toSkip := make([]string, 0)
			for _, t := range branch.Targets() {
				if t != target && t != END {
					toSkip = append(toSkip, t)
				}
			}
			for len(toSkip) > 0 {
				cur := toSkip[0]
				toSkip = toSkip[1:]
				if skipped[cur] {
					continue
				}
				skipped[cur] = true
				for _, s := range r.successors[cur] {
					if s != END && !skipped[s] {
						toSkip = append(toSkip, s)
					}
				}
			}
		}
	}

	// 取直接前驱 END 的节点输出（选第一个非跳过且有输出的）
	for _, pred := range r.predecessors[END] {
		if !skipped[pred] {
			if v, ok := outputs[pred]; ok {
				outputs[END] = v
				break
			}
		}
	}

	r.callback.OnGraphEnd(ctx, graphInfo, outputs[END], nil)
	return outputs, nil
}

// castOutput 将 any 转换为 O.
func castOutput[O any](v any) (O, error) {
	if v == nil {
		var zero O
		return zero, nil
	}
	o, ok := v.(O)
	if !ok {
		var zero O
		return zero, fmt.Errorf("compose: output type mismatch: expected %T, got %T", zero, v)
	}
	return o, nil
}

// compileGraph 编译图：拓扑排序 + 完整性检查.
func compileGraph[I, O any](_ context.Context, g *Graph[I, O], stateInjector func(context.Context) context.Context, callbacks []CallbackHandler) (*Runnable[I, O], error) {
	nodeMap := make(map[string]nodeEntry, len(g.nodes))
	for _, n := range g.nodes {
		nodeMap[n.id] = n
	}

	// 构建前驱/后继表
	successors := make(map[string][]string)
	predecessors := make(map[string][]string)
	for _, e := range g.edges {
		successors[e.from] = append(successors[e.from], e.to)
		predecessors[e.to] = append(predecessors[e.to], e.from)
	}

	branchMap := make(map[string]*Branch)
	for _, be := range g.branches {
		branchMap[be.from] = be.branch
		for _, t := range be.branch.Targets() {
			if t == END {
				predecessors[END] = append(predecessors[END], be.from)
			} else {
				successors[be.from] = append(successors[be.from], t)
				predecessors[t] = append(predecessors[t], be.from)
			}
		}
	}

	// 边引用节点存在性检查
	allKnown := make(map[string]struct{}, len(nodeMap)+2)
	allKnown[START] = struct{}{}
	allKnown[END] = struct{}{}
	for id := range nodeMap {
		allKnown[id] = struct{}{}
	}
	for _, e := range g.edges {
		if _, ok := allKnown[e.from]; !ok {
			return nil, &CompileError{Reason: fmt.Sprintf("edge references unknown node %q", e.from)}
		}
		if _, ok := allKnown[e.to]; !ok {
			return nil, &CompileError{Reason: fmt.Sprintf("edge references unknown node %q", e.to)}
		}
	}

	// 基本完整性检查
	if len(successors[START]) == 0 {
		return nil, &CompileError{Reason: "no edges from START"}
	}
	if len(predecessors[END]) == 0 {
		return nil, &CompileError{Reason: "no edges to END"}
	}

	// Kahn 算法拓扑排序（仅用户节点）
	inDegree := make(map[string]int, len(nodeMap))
	for id := range nodeMap {
		for _, pred := range predecessors[id] {
			if pred != START {
				inDegree[id]++
			}
		}
	}

	var order []string
	queue := make([]string, 0, len(nodeMap))
	for id := range nodeMap {
		if inDegree[id] == 0 {
			queue = append(queue, id)
		}
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		order = append(order, cur)
		for _, succ := range successors[cur] {
			if succ == END {
				continue
			}
			if _, isUser := nodeMap[succ]; !isUser {
				continue
			}
			inDegree[succ]--
			if inDegree[succ] == 0 {
				queue = append(queue, succ)
			}
		}
	}

	if len(order) != len(nodeMap) {
		return nil, &CompileError{Reason: fmt.Sprintf("cycle detected: processed %d/%d nodes", len(order), len(nodeMap))}
	}

	// 可达性检查（BFS 从 START 出发）
	reachable := make(map[string]bool)
	bfsQ := append([]string{}, successors[START]...)
	for len(bfsQ) > 0 {
		cur := bfsQ[0]
		bfsQ = bfsQ[1:]
		if reachable[cur] || cur == END {
			continue
		}
		reachable[cur] = true
		bfsQ = append(bfsQ, successors[cur]...)
		if b, ok := branchMap[cur]; ok {
			bfsQ = append(bfsQ, b.Targets()...)
		}
	}
	for id := range nodeMap {
		if !reachable[id] {
			return nil, &CompileError{Reason: fmt.Sprintf("node %q is unreachable from START", id)}
		}
	}

	return &Runnable[I, O]{
		order:         order,
		nodes:         nodeMap,
		successors:    successors,
		predecessors:  predecessors,
		branches:      branchMap,
		stateInjector: stateInjector,
		callback:      buildCallbackHandler(callbacks),
	}, nil
}

