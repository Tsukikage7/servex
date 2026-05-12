package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/agent/checkpoint"
	"github.com/Tsukikage7/servex/v2/llm/agent/toolcall"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Strategy Agent 执行策略接口.
type Strategy interface {
	// Execute 同步执行，返回最终结果.
	Execute(ctx context.Context, model llm.ChatModel, tools *toolcall.Registry, messages []llm.Message, maxIter int, log logger.Logger, opts ...llm.CallOption) (*Result, error)
	// ExecuteStream 流式执行，通过 channel 返回事件.
	ExecuteStream(ctx context.Context, model llm.ChatModel, tools *toolcall.Registry, messages []llm.Message, maxIter int, log logger.Logger, opts ...llm.CallOption) (<-chan Event, error)
}

// ---------- ReAct 策略 ----------

// reActStrategy ReAct（Reasoning + Acting）策略.
// 利用 toolcall.Executor 实现 "思考 → 行动 → 观察" 循环.
type reActStrategy struct{}

// NewReActStrategy 创建 ReAct 策略.
func NewReActStrategy() Strategy {
	return &reActStrategy{}
}

// Execute 同步执行 ReAct 策略.
func (s *reActStrategy) Execute(ctx context.Context, model llm.ChatModel, tools *toolcall.Registry, messages []llm.Message, maxIter int, log logger.Logger, opts ...llm.CallOption) (*Result, error) {
	log = logger.WithComponent(log, "Agent")

	cb := buildAgentCallbackHandler(getAgentCallbacks(ctx))

	// 无工具时直接调用模型
	if tools == nil {
		cb.OnLLMCall(ctx, messages)
		resp, err := model.Generate(ctx, messages, opts...)
		if err != nil {
			return nil, err
		}
		cb.OnLLMResponse(ctx, resp)
		return &Result{
			Output:     resp.Message.Content,
			Messages:   append(messages, resp.Message),
			Iterations: 1,
			Usage:      resp.Usage,
		}, nil
	}

	// 收集所有工具调用结果
	var allToolResults []toolcall.ToolResult
	var totalUsage llm.Usage

	executorOpts := []toolcall.ExecutorOption{
		toolcall.WithMaxRounds(maxIter),
		toolcall.WithPreToolCallback(func(cbCtx context.Context, call llm.ToolCall) {
			cb.OnToolCallStart(cbCtx, call)
		}),
		toolcall.WithOnStep(func(stepCtx context.Context, event toolcall.StepEvent) {
			if event.Response != nil {
				totalUsage.Add(event.Response.Usage)
				cb.OnLLMResponse(stepCtx, event.Response)
			}
			for _, tr := range event.ToolResults {
				cb.OnToolCallEnd(stepCtx, tr.Call, tr.Output, tr.Err)
			}
			allToolResults = append(allToolResults, event.ToolResults...)
			if log != nil {
				if event.IsFinal {
					log.Debugf("ReAct 策略第 %d 轮完成（最终轮）", event.Round+1)
				} else {
					log.Debugf("ReAct 策略第 %d 轮完成，执行了 %d 个工具调用", event.Round+1, len(event.ToolResults))
				}
			}
		}),
	}

	// 注入中断检查（从 context 取出配置）
	if ic := getInterruptConfig(ctx); ic != nil {
		executorOpts = append(executorOpts, toolcall.WithInterruptCheck(
			func(checkCtx context.Context, toolCalls []llm.ToolCall, msgs []llm.Message, round int) error {
				var needsInterrupt []llm.ToolCall
				for _, tc := range toolCalls {
					if ic.policy.ShouldInterrupt(checkCtx, tc) {
						needsInterrupt = append(needsInterrupt, tc)
					}
				}
				if len(needsInterrupt) == 0 {
					return nil
				}
				cp := &checkpoint.AgentCheckpoint{
					ID:        uuid.New().String(),
					Messages:  msgs,
					ToolCalls: needsInterrupt,
					Iteration: round,
					CreatedAt: time.Now(),
					Metadata:  map[string]any{"agent": ic.agentName},
				}
				if err := ic.store.Save(checkCtx, cp); err != nil {
					return fmt.Errorf("agent: save checkpoint: %w", err)
				}
				return &InterruptError{
					CheckpointID: cp.ID,
					ToolCalls:    needsInterrupt,
					Messages:     msgs,
				}
			},
		))
	}

	executor := toolcall.NewExecutor(model, tools, executorOpts...)

	// 在发起 LLM 调用前触发 OnLLMCall
	cb.OnLLMCall(ctx, messages)
	execResult, err := executor.Run(ctx, messages, opts...)
	if err != nil {
		// 检查是否为最大轮次错误
		if errors.Is(err, toolcall.ErrMaxRounds) {
			return nil, ErrMaxIterations
		}
		return nil, err
	}

	return &Result{
		Output:     execResult.Response.Message.Content,
		Messages:   execResult.Messages,
		ToolCalls:  allToolResults,
		Iterations: execResult.Rounds,
		Usage:      totalUsage,
	}, nil
}

// ExecuteStream 流式执行 ReAct 策略.
func (s *reActStrategy) ExecuteStream(ctx context.Context, model llm.ChatModel, tools *toolcall.Registry, messages []llm.Message, maxIter int, log logger.Logger, opts ...llm.CallOption) (<-chan Event, error) {
	log = logger.WithComponent(log, "Agent")

	ch := make(chan Event, 16)

	go func() {
		defer close(ch)
		defer func() {
			if r := recover(); r != nil {
				ch <- Event{Type: EventError, Content: fmt.Sprintf("goroutine panic: %v", r)}
			}
		}()

		cb := buildAgentCallbackHandler(getAgentCallbacks(ctx))

		// 无工具时直接调用模型流式接口
		if tools == nil {
			cb.OnLLMCall(ctx, messages)
			reader, err := model.Stream(ctx, messages, opts...)
			if err != nil {
				ch <- Event{Type: EventError, Content: err.Error()}
				return
			}
			defer reader.Close()
			var content string
			for {
				chunk, recvErr := reader.Recv()
				if recvErr != nil {
					if recvErr == io.EOF {
						break
					}
					ch <- Event{Type: EventError, Content: recvErr.Error()}
					return
				}
				if chunk.Delta != "" {
					ch <- Event{Type: EventToken, Content: chunk.Delta}
					content += chunk.Delta
				}
			}
			// 构造最终响应触发 OnLLMResponse
			finalResp := reader.Response()
			if finalResp != nil {
				cb.OnLLMResponse(ctx, finalResp)
			}
			ch <- Event{Type: EventOutput, Content: content}
			return
		}

		cb.OnLLMCall(ctx, messages)
		executor := toolcall.NewExecutor(model, tools,
			toolcall.WithMaxRounds(maxIter),
			toolcall.WithStreamCallback(func(streamCtx context.Context, chunk llm.StreamChunk) error {
				if chunk.Delta != "" {
					ch <- Event{Type: EventToken, Content: chunk.Delta}
				}
				return nil
			}),
			toolcall.WithOnStep(func(stepCtx context.Context, event toolcall.StepEvent) {
				if event.Response != nil {
					cb.OnLLMResponse(stepCtx, event.Response)
				}
				if !event.IsFinal {
					if event.Response.Message.Content != "" {
						ch <- Event{Type: EventThinking, Content: event.Response.Message.Content}
					}
					for i, tc := range event.Response.Message.ToolCalls {
						tcCopy := tc
						cb.OnToolCallStart(stepCtx, tcCopy)
						ch <- Event{Type: EventToolCall, ToolCall: &tcCopy}
						if i < len(event.ToolResults) {
							trCopy := event.ToolResults[i]
							cb.OnToolCallEnd(stepCtx, tcCopy, trCopy.Output, trCopy.Err)
							ch <- Event{Type: EventToolResult, ToolResult: &trCopy}
						}
					}
				}
			}),
		)

		execResult, err := executor.Run(ctx, messages, opts...)
		if err != nil {
			if errors.Is(err, toolcall.ErrMaxRounds) {
				ch <- Event{Type: EventError, Content: ErrMaxIterations.Error()}
			} else {
				ch <- Event{Type: EventError, Content: err.Error()}
			}
			return
		}
		ch <- Event{Type: EventOutput, Content: execResult.Response.Message.Content}
	}()

	return ch, nil
}

// ---------- PlanExecute 策略 ----------

// planExecuteStrategy 计划-执行策略.
// 先让模型将任务分解为步骤列表，再逐步执行.
type planExecuteStrategy struct{}

// NewPlanExecuteStrategy 创建 PlanExecute 策略.
func NewPlanExecuteStrategy() Strategy {
	return &planExecuteStrategy{}
}

// planPromptTemplate 计划提示词模板.
const planPromptTemplate = `请将以下任务分解为步骤列表，输出 JSON 数组（每个元素为字符串描述一个步骤），不要包含其他内容。

任务：%s`

// Execute 同步执行 PlanExecute 策略.
func (s *planExecuteStrategy) Execute(ctx context.Context, model llm.ChatModel, tools *toolcall.Registry, messages []llm.Message, maxIter int, log logger.Logger, opts ...llm.CallOption) (*Result, error) {
	log = logger.WithComponent(log, "Agent")

	// 提取用户输入（最后一条用户消息）
	userInput := extractUserInput(messages)

	// 第一步：让模型生成计划
	planMessages := []llm.Message{
		llm.SystemMessage("你是一个任务规划专家，请将任务分解为可执行的步骤列表。"),
		llm.UserMessage(fmt.Sprintf(planPromptTemplate, userInput)),
	}

	if log != nil {
		log.Debugf("PlanExecute 策略开始生成计划")
	}

	planResp, err := model.Generate(ctx, planMessages, opts...)
	if err != nil {
		return nil, fmt.Errorf("agent: 生成计划失败: %w", err)
	}

	// 解析步骤列表
	steps, err := parsePlanSteps(planResp.Message.Content)
	if err != nil {
		return nil, fmt.Errorf("agent: 解析计划失败: %w", err)
	}

	if log != nil {
		log.Debugf("PlanExecute 策略计划包含 %d 个步骤", len(steps))
	}

	// 第二步：逐步执行
	var totalUsage llm.Usage
	totalUsage.Add(planResp.Usage)
	var allToolResults []toolcall.ToolResult
	allMessages := make([]llm.Message, len(messages))
	copy(allMessages, messages)

	var lastOutput string
	iterations := 0

	for i, step := range steps {
		if iterations >= maxIter {
			return nil, ErrMaxIterations
		}

		if log != nil {
			log.With(logger.String("step", step)).Debugf("PlanExecute 策略执行步骤 %d/%d", i+1, len(steps))
		}

		// 构建步骤消息
		stepMessages := []llm.Message{
			llm.SystemMessage("请执行以下步骤，如有需要可使用可用工具。"),
			llm.UserMessage(step),
		}

		// 如果有工具，使用 Executor
		if tools != nil {
			executor := toolcall.NewExecutor(model, tools,
				toolcall.WithMaxRounds(maxIter-iterations),
				toolcall.WithOnStep(func(_ context.Context, event toolcall.StepEvent) {
					if event.Response != nil {
						totalUsage.Add(event.Response.Usage)
					}
					allToolResults = append(allToolResults, event.ToolResults...)
				}),
			)

			execResult, execErr := executor.Run(ctx, stepMessages, opts...)
			if execErr != nil {
				if errors.Is(execErr, toolcall.ErrMaxRounds) {
					return nil, ErrMaxIterations
				}
				return nil, fmt.Errorf("agent: 步骤 %d 执行失败: %w", i+1, execErr)
			}
			lastOutput = execResult.Response.Message.Content
			iterations += execResult.Rounds + 1
			allMessages = append(allMessages, execResult.Messages...)
		} else {
			// 无工具直接调用模型
			resp, genErr := model.Generate(ctx, stepMessages, opts...)
			if genErr != nil {
				return nil, fmt.Errorf("agent: 步骤 %d 执行失败: %w", i+1, genErr)
			}
			lastOutput = resp.Message.Content
			totalUsage.Add(resp.Usage)
			iterations++
			allMessages = append(allMessages, stepMessages...)
			allMessages = append(allMessages, resp.Message)
		}
	}

	return &Result{
		Output:     lastOutput,
		Messages:   allMessages,
		ToolCalls:  allToolResults,
		Iterations: iterations,
		Usage:      totalUsage,
	}, nil
}

// ExecuteStream 流式执行 PlanExecute 策略.
func (s *planExecuteStrategy) ExecuteStream(ctx context.Context, model llm.ChatModel, tools *toolcall.Registry, messages []llm.Message, maxIter int, log logger.Logger, opts ...llm.CallOption) (<-chan Event, error) {
	log = logger.WithComponent(log, "Agent")

	ch := make(chan Event, 16)

	go func() {
		defer close(ch)
		defer func() {
			if r := recover(); r != nil {
				ch <- Event{Type: EventError, Content: fmt.Sprintf("goroutine panic: %v", r)}
			}
		}()

		// 复用同步执行逻辑，将中间过程以事件方式发送
		userInput := extractUserInput(messages)

		// 生成计划
		planMessages := []llm.Message{
			llm.SystemMessage("你是一个任务规划专家，请将任务分解为可执行的步骤列表。"),
			llm.UserMessage(fmt.Sprintf(planPromptTemplate, userInput)),
		}

		planResp, err := model.Generate(ctx, planMessages, opts...)
		if err != nil {
			ch <- Event{Type: EventError, Content: err.Error()}
			return
		}

		ch <- Event{Type: EventThinking, Content: "计划生成完成: " + planResp.Message.Content}

		steps, err := parsePlanSteps(planResp.Message.Content)
		if err != nil {
			ch <- Event{Type: EventError, Content: fmt.Sprintf("解析计划失败: %v", err)}
			return
		}

		var lastOutput string
		iterations := 0

		for i, step := range steps {
			if iterations >= maxIter {
				ch <- Event{Type: EventError, Content: ErrMaxIterations.Error()}
				return
			}

			ch <- Event{Type: EventThinking, Content: fmt.Sprintf("执行步骤 %d/%d: %s", i+1, len(steps), step)}

			stepMessages := []llm.Message{
				llm.SystemMessage("请执行以下步骤，如有需要可使用可用工具。"),
				llm.UserMessage(step),
			}

			if tools != nil {
				executor := toolcall.NewExecutor(model, tools,
					toolcall.WithMaxRounds(maxIter-iterations),
					toolcall.WithOnStep(func(_ context.Context, event toolcall.StepEvent) {
						if !event.IsFinal {
							for j, tc := range event.Response.Message.ToolCalls {
								tcCopy := tc
								ch <- Event{Type: EventToolCall, ToolCall: &tcCopy}
								if j < len(event.ToolResults) {
									trCopy := event.ToolResults[j]
									ch <- Event{Type: EventToolResult, ToolResult: &trCopy}
								}
							}
						}
					}),
				)

				execResult, execErr := executor.Run(ctx, stepMessages, opts...)
				if execErr != nil {
					ch <- Event{Type: EventError, Content: execErr.Error()}
					return
				}
				lastOutput = execResult.Response.Message.Content
				iterations += execResult.Rounds + 1
			} else {
				resp, genErr := model.Generate(ctx, stepMessages, opts...)
				if genErr != nil {
					ch <- Event{Type: EventError, Content: genErr.Error()}
					return
				}
				lastOutput = resp.Message.Content
				iterations++
			}
		}

		ch <- Event{Type: EventOutput, Content: lastOutput}
	}()

	return ch, nil
}

// extractUserInput 提取消息列表中最后一条用户消息的内容.
func extractUserInput(messages []llm.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == llm.RoleUser {
			return messages[i].Content
		}
	}
	return ""
}

// parsePlanSteps 解析 JSON 数组格式的步骤列表.
// 支持模型返回内容中包含 markdown 代码块的情况.
func parsePlanSteps(content string) ([]string, error) {
	// 去除可能的 markdown 代码块标记
	cleaned := strings.TrimSpace(content)
	if strings.HasPrefix(cleaned, "```") {
		// 去除首行 ```json 和末尾 ```
		lines := strings.Split(cleaned, "\n")
		if len(lines) >= 2 {
			cleaned = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}
	cleaned = strings.TrimSpace(cleaned)

	var steps []string
	if err := json.Unmarshal([]byte(cleaned), &steps); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w, 原始内容: %s", err, content)
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("步骤列表为空")
	}
	return steps, nil
}
