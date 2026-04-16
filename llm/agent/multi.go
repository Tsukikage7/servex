package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/agent/toolcall"
	"github.com/Tsukikage7/servex/v2/xutil/syncx"
)

// Supervisor 监督者模式.
// 一个监督者 Agent 负责分配任务给多个工作者 Agent，通过工具调用机制实现委派.
type Supervisor struct {
	supervisor *Agent
	workers    map[string]*Agent
}

// NewSupervisor 创建监督者模式多 Agent 协作.
// supervisor 为监督者 Agent，workers 为命名的工作者 Agent 映射.
// 每个 worker 会自动注册为监督者的工具，工具名即 worker 名称.
// 注意：workers map 的迭代顺序不确定，因此工具注册顺序可能随运行变化.
func NewSupervisor(supervisor *Agent, workers map[string]*Agent) *Supervisor {
	return &Supervisor{
		supervisor: supervisor,
		workers:    workers,
	}
}

// Run 执行监督者模式.
// 将每个 worker 注册为监督者的工具，监督者通过工具调用来委派任务给 worker.
func (s *Supervisor) Run(ctx context.Context, input string, opts ...llm.CallOption) (*Result, error) {
	// 创建工具注册表（复用或新建）
	registry := toolcall.NewRegistry()

	// 将已有工具注册进来
	if s.supervisor.cfg.Tools != nil {
		for _, t := range s.supervisor.cfg.Tools.Tools() {
			name := t.Function.Name
			registry.Register(t, func(execCtx context.Context, args string) (string, error) {
				// callID 在 HandlerFunc 签名中不可见，传空字符串；
				// Registry.Execute 目前忽略 callID，不影响正确性.
				return s.supervisor.cfg.Tools.Execute(execCtx, "", name, args)
			})
		}
	}

	// 将每个 worker 注册为工具
	for name, worker := range s.workers {
		w := worker // 捕获循环变量
		tool := llm.Tool{
			Function: llm.FunctionDef{
				Name:        name,
				Description: fmt.Sprintf("委派任务给 %s 工作者 Agent", name),
				Parameters:  json.RawMessage(`{"type":"object","properties":{"task":{"type":"string","description":"要委派的任务描述"}},"required":["task"]}`),
			},
		}
		registry.Register(tool, func(execCtx context.Context, arguments string) (string, error) {
			var args struct {
				Task string `json:"task"`
			}
			if err := json.Unmarshal([]byte(arguments), &args); err != nil {
				return "", fmt.Errorf("agent: 解析委派参数失败: %w", err)
			}
			result, err := w.Run(execCtx, args.Task, opts...)
			if err != nil {
				return "", err
			}
			return result.Output, nil
		})
	}

	// 替换监督者的工具注册表
	supervisorCfg := s.supervisor.cfg
	supervisorCfg.Tools = registry
	supervisorAgent, err := New(&supervisorCfg)
	if err != nil {
		return nil, err
	}

	return supervisorAgent.Run(ctx, input, opts...)
}

// Pipeline 管道模式.
// 多个 Agent 串行执行，前一个的输出作为后一个的输入.
type Pipeline struct {
	agents []*Agent
}

// NewPipeline 创建管道模式多 Agent 协作.
// agents 按执行顺序传入，第一个 Agent 接收原始输入，后续每个接收前一个的输出.
func NewPipeline(agents ...*Agent) *Pipeline {
	return &Pipeline{agents: agents}
}

// Run 执行管道.
// 按顺序执行每个 Agent，累积总用量，收集所有工具调用.
func (p *Pipeline) Run(ctx context.Context, input string, opts ...llm.CallOption) (*Result, error) {
	if len(p.agents) == 0 {
		return &Result{Output: input}, nil
	}

	currentInput := input
	var totalUsage llm.Usage
	var allToolCalls []toolcall.ToolResult
	var allMessages []llm.Message
	totalIterations := 0

	for i, agent := range p.agents {
		result, err := agent.Run(ctx, currentInput, opts...)
		if err != nil {
			return nil, fmt.Errorf("agent: 管道第 %d 步执行失败: %w", i+1, err)
		}

		currentInput = result.Output
		totalUsage.Add(result.Usage)
		allToolCalls = append(allToolCalls, result.ToolCalls...)
		allMessages = append(allMessages, result.Messages...)
		totalIterations += result.Iterations
	}

	return &Result{
		Output:     currentInput,
		Messages:   allMessages,
		ToolCalls:  allToolCalls,
		Iterations: totalIterations,
		Usage:      totalUsage,
	}, nil
}

// FanOut 并行 Fan-out/Fan-in 多 Agent 模式.
// 将同一 input 并发分发给所有 Agent，全部完成后返回各自结果.
// 任意 Agent 失败则取消其余并返回错误.
type FanOut struct {
	agents map[string]*Agent
}

// NewFanOut 创建 FanOut 实例.
func NewFanOut(agents map[string]*Agent) *FanOut {
	return &FanOut{agents: agents}
}

// Run 并发执行所有 Agent，返回 agentName → Result 映射.
func (f *FanOut) Run(ctx context.Context, input string, opts ...llm.CallOption) (map[string]*Result, error) {
	g, gCtx := errgroup.WithContext(ctx)
	var results syncx.Map[string, *Result]

	for name, a := range f.agents {
		name, a := name, a
		g.Go(func() error {
			r, err := a.Run(gCtx, input, opts...)
			if err != nil {
				return fmt.Errorf("agent %q: %w", name, err)
			}
			results.Store(name, r)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	out := make(map[string]*Result, len(f.agents))
	results.Range(func(k string, v *Result) bool {
		out[k] = v
		return true
	})
	return out, nil
}

// Blackboard 线程安全的黑板共享状态，用于多 Agent 间松散数据共享.
type Blackboard struct {
	data syncx.Map[string, any]
}

// NewBlackboard 创建黑板实例.
func NewBlackboard() *Blackboard {
	return &Blackboard{}
}

// Set 写入键值对.
func (b *Blackboard) Set(key string, value any) {
	b.data.Store(key, value)
}

// Get 读取值，不存在时返回 nil, false.
func (b *Blackboard) Get(key string) (any, bool) {
	return b.data.Load(key)
}

// GetFromBoard 泛型类型安全读取.
// 键不存在或类型不匹配时返回零值和 false.
func GetFromBoard[T any](b *Blackboard, key string) (T, bool) {
	v, ok := b.data.Load(key)
	if !ok {
		var zero T
		return zero, false
	}
	typed, ok := v.(T)
	return typed, ok
}

// BlackboardAgent 包装 Agent，执行前从黑板注入上下文，执行后将输出写回黑板.
type BlackboardAgent struct {
	agent     *Agent
	board     *Blackboard
	inputKey  string
	outputKey string
}

// NewBlackboardAgent 创建黑板 Agent 包装器.
// inputKey：从黑板读取此 key 的值追加到 input 末尾（空则不读）.
// outputKey：将 Agent 输出写入黑板此 key（空则不写）.
func NewBlackboardAgent(a *Agent, board *Blackboard, inputKey, outputKey string) *BlackboardAgent {
	return &BlackboardAgent{agent: a, board: board, inputKey: inputKey, outputKey: outputKey}
}

// Run 执行 Agent，自动注入/写出黑板数据.
func (a *BlackboardAgent) Run(ctx context.Context, input string, opts ...llm.CallOption) (*Result, error) {
	augmented := input
	if a.inputKey != "" {
		if v, ok := a.board.Get(a.inputKey); ok {
			augmented = fmt.Sprintf("%s\n\n[Context: %v]", input, v)
		}
	}

	result, err := a.agent.Run(ctx, augmented, opts...)
	if err != nil {
		return nil, err
	}

	if a.outputKey != "" {
		a.board.Set(a.outputKey, result.Output)
	}

	return result, nil
}
