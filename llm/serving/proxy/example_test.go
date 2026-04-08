package proxy_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/llm"
	"github.com/Tsukikage7/servex/llm/provider/openai"
	"github.com/Tsukikage7/servex/llm/serving/proxy"
)

func ExampleNew() {
	client := openai.New("sk-test")
	p := proxy.New(map[string]llm.ChatModel{
		"openai": client,
	})
	fmt.Println(p != nil)
	// Output:
	// true
}
