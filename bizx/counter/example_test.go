package counter_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/bizx/counter"
)

func ExampleNewMemoryCounter() {
	c := counter.NewMemoryCounter(counter.WithPrefix("app:"))
	ctx := context.Background()

	// 递增计数.
	val, _ := c.Incr(ctx, "login_count", 1)
	fmt.Println("after first incr:", val)

	val, _ = c.Incr(ctx, "login_count", 5)
	fmt.Println("after second incr:", val)

	// 获取计数.
	val, _ = c.Get(ctx, "login_count")
	fmt.Println("get:", val)

	// 重置.
	_ = c.Reset(ctx, "login_count")
	val, _ = c.Get(ctx, "login_count")
	fmt.Println("after reset:", val)
	// Output:
	// after first incr: 1
	// after second incr: 6
	// get: 6
	// after reset: 0
}

func ExampleCounter_MGet() {
	c := counter.NewMemoryCounter()
	ctx := context.Background()

	c.Incr(ctx, "a", 10)
	c.Incr(ctx, "b", 20)
	c.Incr(ctx, "c", 30)

	vals, _ := c.MGet(ctx, "a", "b", "c")
	fmt.Println("a:", vals["a"])
	fmt.Println("b:", vals["b"])
	fmt.Println("c:", vals["c"])
	// Output:
	// a: 10
	// b: 20
	// c: 30
}
