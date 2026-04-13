package openai_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm/provider/openai"
)

func ExampleNew() {
	client := openai.New("sk-test",
		openai.WithModel("gpt-4o"),
		openai.WithBaseURL("https://api.openai.com/v1"),
	)
	fmt.Println(client != nil)
	// Output:
	// true
}
