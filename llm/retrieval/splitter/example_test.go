package splitter_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/llm/retrieval/splitter"
)

func ExampleNewCharacterSplitter() {
	s := splitter.NewCharacterSplitter(
		splitter.WithChunkSize(10),
		splitter.WithChunkOverlap(2),
	)
	chunks := s.Split("Hello world, this is a test.")
	fmt.Println(len(chunks) > 0)
	fmt.Println(chunks[0].Index)
	// Output:
	// true
	// 0
}
