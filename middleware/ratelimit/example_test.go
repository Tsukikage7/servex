package ratelimit_test

import (
	"context"
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/middleware/ratelimit"
)

func ExampleNewTokenBucket() {
	// 创建令牌桶: 每秒 10 个令牌，桶容量 10.
	limiter := ratelimit.NewTokenBucket(10, 10)

	ctx := context.Background()

	// 初始状态满桶，前 10 个请求均允许通过.
	allowed := 0
	for range 12 {
		if limiter.Allow(ctx) {
			allowed++
		}
	}
	fmt.Println("allowed:", allowed)
	// Output: allowed: 10
}

func ExampleNewSlidingWindow() {
	// 创建滑动窗口: 1 秒内最多 3 个请求.
	limiter := ratelimit.NewSlidingWindow(3, time.Second)

	ctx := context.Background()

	results := make([]bool, 5)
	for i := range 5 {
		results[i] = limiter.Allow(ctx)
	}
	fmt.Println(results[0], results[1], results[2]) // 前 3 个通过
	fmt.Println(results[3], results[4])              // 后 2 个被限流
	// Output:
	// true true true
	// false false
}

func ExampleNewFixedWindow() {
	// 创建固定窗口: 1 秒内最多 2 个请求.
	limiter := ratelimit.NewFixedWindow(2, time.Second)

	ctx := context.Background()

	fmt.Println(limiter.Allow(ctx))
	fmt.Println(limiter.Allow(ctx))
	fmt.Println(limiter.Allow(ctx))
	// Output:
	// true
	// true
	// false
}
