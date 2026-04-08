package vectorstore_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/llm/retrieval/vectorstore"
)

func ExampleWithFilter() {
	opt := vectorstore.WithFilter(map[string]any{"category": "tech"})
	fmt.Println(opt != nil)
	// Output:
	// true
}

func ExampleWithScoreThreshold() {
	opt := vectorstore.WithScoreThreshold(0.8)
	fmt.Println(opt != nil)
	// Output:
	// true
}
