package memory_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm/agent/memory"
)

func ExampleNewMemoryStore() {
	store := memory.NewMemoryStore()
	fmt.Println(store != nil)
	// Output:
	// true
}
