package rag_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm/retrieval/rag"
)

func ExampleDocument() {
	doc := rag.Document{
		ID:      "doc-1",
		Content: "Go is a statically typed language.",
	}
	fmt.Println(doc.ID)
	fmt.Println(doc.Content)
	// Output:
	// doc-1
	// Go is a statically typed language.
}
