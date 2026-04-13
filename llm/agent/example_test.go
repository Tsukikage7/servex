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
