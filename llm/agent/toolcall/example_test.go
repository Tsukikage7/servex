package toolcall_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/agent/toolcall"
)

func ExampleNewRegistry() {
	reg := toolcall.NewRegistry()
	reg.Register(llm.Tool{
		Function: llm.FunctionDef{
			Name:        "get_weather",
			Description: "Get weather for a city",
		},
	}, func(_ context.Context, _ string) (string, error) {
		return `{"temp": 22}`, nil
	})
	fmt.Println(len(reg.Tools()))
	// Output:
	// 1
}
