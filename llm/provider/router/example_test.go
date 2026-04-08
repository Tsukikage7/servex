package router_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/llm/provider/openai"
	"github.com/Tsukikage7/servex/llm/provider/router"
)

func ExampleNew() {
	fallback := openai.New("sk-test")
	r := router.New(fallback, router.Route{
		Models: []string{"gpt-4o"},
		Model:  fallback,
	})
	fmt.Println(r != nil)
	// Output:
	// true
}
