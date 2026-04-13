package tokenizer_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm/processing/tokenizer"
)

func ExampleEstimateTokens() {
	count := tokenizer.EstimateTokens("Hello, world!")
	fmt.Println(count > 0)
	// Output:
	// true
}

func ExampleNewEstimateTokenizer() {
	tok := tokenizer.NewEstimateTokenizer()
	n := tok.Count("Hello, world!")
	fmt.Println(n > 0)
	// Output:
	// true
}
