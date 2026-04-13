package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/agent/toolcall"
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
