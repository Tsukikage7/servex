package blockingqueue_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/collections/blockingqueue"
)

func ExampleNew() {
	q := blockingqueue.New[string](10)
	ctx := context.Background()

	_ = q.Enqueue(ctx, "first")
	_ = q.Enqueue(ctx, "second")
	_ = q.Enqueue(ctx, "third")

	fmt.Println("len:", q.Len())

	v1, _ := q.Dequeue(ctx)
	v2, _ := q.Dequeue(ctx)
	fmt.Println(v1)
	fmt.Println(v2)
	fmt.Println("len after dequeue:", q.Len())
	// Output:
	// len: 3
	// first
	// second
	// len after dequeue: 1
}
