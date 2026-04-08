package gemini_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/llm/provider/gemini"
)

func ExampleNew() {
	client := gemini.New("test-key",
		gemini.WithModel("gemini-pro"),
	)
	fmt.Println(client != nil)
	// Output:
	// true
}
