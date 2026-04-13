package anthropic_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm/provider/anthropic"
)

func ExampleNew() {
	client := anthropic.New("sk-test",
		anthropic.WithModel("claude-sonnet-4-20250514"),
		anthropic.WithDefaultMaxTokens(1024),
	)
	fmt.Println(client != nil)
	// Output:
	// true
}
