package apikey_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm/gateway/apikey"
)

func ExampleNewMemoryStore() {
	store := apikey.NewMemoryStore()
	fmt.Println(store != nil)
	// Output:
	// true
}

func ExampleNewContext() {
	key := &apikey.Key{ID: "key-1"}
	ctx := apikey.NewContext(context.Background(), key)
	got, ok := apikey.FromContext(ctx)
	fmt.Println(ok)
	fmt.Println(got.ID)
	// Output:
	// true
	// key-1
}
