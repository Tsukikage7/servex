package cache_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/llm/serving/cache"
)

func ExampleNewMemoryStore() {
	store := cache.NewMemoryStore()
	fmt.Println(store != nil)
	// Output:
	// true
}
