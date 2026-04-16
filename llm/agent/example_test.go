package agent_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm/agent"
)

func ExampleConfig() {
	cfg := agent.Config{
		Name:          "assistant",
		SystemPrompt:  "You are a helpful assistant.",
		MaxIterations: 5,
	}
	fmt.Println(cfg.Name)
	fmt.Println(cfg.MaxIterations)
	fmt.Println(agent.ErrNilModel)
	// Output:
	// assistant
	// 5
	// agent: model is nil
}

func ExampleAgent_RunStream_token() {
	// 演示逐 token 接收输出（实际使用时替换为真实模型）
	// ch, _ := myAgent.RunStream(ctx, "帮我分析数据")
	// for evt := range ch {
	//     if evt.Type == agent.EventToken {
	//         fmt.Print(evt.Content)
	//     }
	// }
}

func ExampleFanOut() {
	// 演示并行执行多个 Agent（实际使用时替换为真实模型）
	// fanout := agent.NewFanOut(map[string]*agent.Agent{"a": agentA, "b": agentB})
	// results, _ := fanout.Run(ctx, "分析这份报告")
	// fmt.Println(results["a"].Output)
	_ = agent.NewBlackboard()
}

func ExampleBlackboard() {
	// 演示黑板共享状态
	// board := agent.NewBlackboard()
	// board.Set("context", "项目背景：...")
	// ba := agent.NewBlackboardAgent(analyst, board, "context", "analysis")
	// ba.Run(ctx, "分析用户数据")
	board := agent.NewBlackboard()
	board.Set("key", "value")
	v, _ := board.Get("key")
	_ = v
}
