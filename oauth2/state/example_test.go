package state_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/oauth2/state"
)

func ExampleNewMemoryStore() {
	store := state.NewMemoryStore()
	defer store.Close()
	token, err := store.Generate(context.Background())
	fmt.Println(err)
	fmt.Println(len(token) > 0)
	// Output:
	// <nil>
	// true
}
