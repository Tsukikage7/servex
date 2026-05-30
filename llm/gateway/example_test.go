package gateway_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/gateway"
	"github.com/Tsukikage7/servex/v2/llm/provider/openai"
)

func ExampleNew() {
	client := openai.New("sk-test")
	p := gateway.New(map[string]llm.ChatModel{
		"openai": client,
	})
	fmt.Println(p != nil)
	// Output:
	// true
}
