package rerank_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/llm/retrieval/rerank"
)

func ExampleWithTopN() {
	opt := rerank.WithTopN(5)
	fmt.Println(opt != nil)
	// Output:
	// true
}
