package timeout_test

import (
	"context"
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/middleware/timeout"
)

func ExampleRemaining() {
	// Context without deadline.
	_, ok := timeout.Remaining(context.Background())
	fmt.Println("has deadline:", ok)

	// Context with deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	remaining, ok := timeout.Remaining(ctx)
	fmt.Println("has deadline:", ok)
	fmt.Println("remaining > 0:", remaining > 0)
	// Output:
	// has deadline: false
	// has deadline: true
	// remaining > 0: true
}

func ExampleCascade() {
	parent, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Cascade takes min(parent remaining, requested timeout).
	ctx, cascadeCancel := timeout.Cascade(parent, 2*time.Second)
	defer cascadeCancel()

	remaining, ok := timeout.Remaining(ctx)
	fmt.Println("has deadline:", ok)
	// 2s is less than parent's ~10s, so remaining should be ~2s.
	fmt.Println("remaining <= 2s:", remaining <= 2*time.Second)
	// Output:
	// has deadline: true
	// remaining <= 2s: true
}

func ExampleWithTimeout() {
	ctx, cancel, err := timeout.WithTimeout(context.Background(), time.Second)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer cancel()

	_, ok := ctx.Deadline()
	fmt.Println("has deadline:", ok)

	// Invalid timeout.
	_, _, err = timeout.WithTimeout(context.Background(), 0)
	fmt.Println("error:", err)
	// Output:
	// has deadline: true
	// error: timeout: 超时时间必须大于0
}
