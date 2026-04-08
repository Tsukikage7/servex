package ratelimit_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/bizx/ratelimit"
)

func ExampleNewMemoryQuotaManager() {
	mgr := ratelimit.NewMemoryQuotaManager()
	ctx := context.Background()

	quota := ratelimit.Quota{
		Key:    "user:123",
		Limit:  5,
		Window: 24 * time.Hour,
	}

	// 消耗配额.
	usage, _ := mgr.Consume(ctx, quota, 3)
	fmt.Println("used:", usage.Used)
	fmt.Println("remaining:", usage.Remaining)

	// 再次消耗.
	usage, _ = mgr.Consume(ctx, quota, 2)
	fmt.Println("used:", usage.Used)
	fmt.Println("remaining:", usage.Remaining)

	// 超限.
	_, err := mgr.Consume(ctx, quota, 1)
	fmt.Println("exceeded:", errors.Is(err, ratelimit.ErrQuotaExceeded))
	// Output:
	// used: 3
	// remaining: 2
	// used: 5
	// remaining: 0
	// exceeded: true
}
