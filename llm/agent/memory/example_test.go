package memory_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/llm/agent/memory"
)

func ExampleNewMemoryStore() {
	store := memory.NewMemoryStore()
	fmt.Println(store != nil)
	// Output:
	// true
}
