package cache_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm/gateway/cache"
)

func ExampleNewMemoryStore() {
	store := cache.NewMemoryStore()
	fmt.Println(store != nil)
	// Output:
	// true
}
