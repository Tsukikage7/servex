package sequence_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/Tsukikage7/servex/v2/bizx/sequence"
)

func ExampleNew() {
	store := sequence.NewMemoryStore()
	seq := sequence.New(&sequence.Config{
		Name:    "order",
		Prefix:  "ORD-",
		Padding: 4,
	}, store)
	ctx := context.Background()

	id1, _ := seq.Next(ctx)
	id2, _ := seq.Next(ctx)
	id3, _ := seq.Next(ctx)

	fmt.Println(id1)
	fmt.Println(id2)
	fmt.Println(id3)
	// Output:
	// ORD-0001
	// ORD-0002
	// ORD-0003
}

func ExampleSequence_Reset() {
	store := sequence.NewMemoryStore()
	seq := sequence.New(&sequence.Config{
		Name:    "ticket",
		Prefix:  "T",
		Padding: 3,
	}, store)
	ctx := context.Background()

	seq.Next(ctx)
	seq.Next(ctx)

	_ = seq.Reset(ctx)

	id, _ := seq.Next(ctx)
	// 去掉可能的日期部分后检查序号.
	fmt.Println(strings.HasSuffix(id, "001"))
	// Output:
	// true
}
