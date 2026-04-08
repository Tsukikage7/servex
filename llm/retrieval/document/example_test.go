package document_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/Tsukikage7/servex/llm/retrieval/document"
)

func ExampleNewTextLoader() {
	loader := document.NewTextLoader(strings.NewReader("Hello, world!"))
	docs, err := loader.Load(context.Background())
	fmt.Println(err)
	fmt.Println(len(docs))
	fmt.Println(docs[0].Content)
	// Output:
	// <nil>
	// 1
	// Hello, world!
}
