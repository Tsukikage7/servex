package memory_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/notify/webhook"
	"github.com/Tsukikage7/servex/v2/notify/webhook/store/memory"
)

func ExampleNewStore() {
	store := memory.NewStore()
	sub := &webhook.Subscription{
		ID:     "sub-1",
		URL:    "https://example.com/webhook",
		Events: []string{"order.created"},
	}
	_ = store.Save(context.Background(), sub)
	got, err := store.Get(context.Background(), "sub-1")
	fmt.Println(err)
	fmt.Println(got.URL)
	// Output:
	// <nil>
	// https://example.com/webhook
}
