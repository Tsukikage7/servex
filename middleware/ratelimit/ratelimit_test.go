package ratelimit

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func TestTokenBucket(t *testing.T) {
	t.Run("基本限流", func(t *testing.T) {
		// 每秒2个令牌，桶容量2
		limiter := NewTokenBucket(2, 2)
		ctx := t.Context()

		// 初始应该可以通过2个请求
		if !limiter.Allow(ctx) {
			t.Error("第一个请求应该通过")
		}
		if !limiter.Allow(ctx) {
			t.Error("第二个请求应该通过")
		}
		// 第三个应该被限流
		if limiter.Allow(ctx) {
			t.Error("第三个请求应该被限流")
		}
	})

	t.Run("令牌补充", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			limiter := NewTokenBucket(10, 2)
			ctx := t.Context()

			// 消耗所有令牌
			limiter.Allow(ctx)
			limiter.Allow(ctx)

			// synctest 虚拟时钟：立即推进，无需真实等待
			time.Sleep(200 * time.Millisecond)

			// 应该可以通过
			if !limiter.Allow(ctx) {
				t.Error("等待后应该有令牌")
			}
		})
	})

	t.Run("AllowN", func(t *testing.T) {
		limiter := NewTokenBucket(10, 5)
		ctx := t.Context()

		if !limiter.AllowN(ctx, 3) {
			t.Error("应该允许3个请求")
		}
		if !limiter.AllowN(ctx, 2) {
			t.Error("应该允许2个请求")
		}
		if limiter.AllowN(ctx, 1) {
			t.Error("应该拒绝请求")
		}
	})

	t.Run("Wait", func(t *testing.T) {
		limiter := NewTokenBucket(100, 1)
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()

		// 消耗令牌
		limiter.Allow(ctx)

		// Wait 应该在超时前成功
		start := time.Now()
		err := limiter.Wait(ctx)
		duration := time.Since(start)

		if err != nil {
			t.Errorf("Wait 不应该失败: %v", err)
		}
		if duration > 50*time.Millisecond {
			t.Errorf("Wait 等待时间过长: %v", duration)
		}
	})

	t.Run("Wait超时", func(t *testing.T) {
		limiter := NewTokenBucket(0.1, 1) // 每10秒1个令牌
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		// 消耗令牌
		limiter.Allow(ctx)

		// Wait 应该超时
		err := limiter.Wait(ctx)
		if err != context.DeadlineExceeded {
			t.Errorf("期望 DeadlineExceeded，得到 %v", err)
		}
	})
}

func TestSlidingWindow(t *testing.T) {
	t.Run("基本限流", func(t *testing.T) {
		limiter := NewSlidingWindow(3, 100*time.Millisecond)
		ctx := t.Context()

		for i := 0; i < 3; i++ {
			if !limiter.Allow(ctx) {
				t.Errorf("第 %d 个请求应该通过", i+1)
			}
		}
		if limiter.Allow(ctx) {
			t.Error("第4个请求应该被限流")
		}
	})

	t.Run("窗口滑动", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			limiter := NewSlidingWindow(2, 100*time.Millisecond)
			ctx := t.Context()

			limiter.Allow(ctx)
			limiter.Allow(ctx)

			// synctest 虚拟时钟：立即推进，无需真实等待
			time.Sleep(150 * time.Millisecond)

			if !limiter.Allow(ctx) {
				t.Error("窗口过期后应该允许请求")
			}
		})
	})

	t.Run("并发安全", func(t *testing.T) {
		limiter := NewSlidingWindow(100, time.Second)
		ctx := t.Context()

		var wg sync.WaitGroup
		allowed := 0
		var mu sync.Mutex

		for i := 0; i < 150; i++ {
			wg.Go(func() {
				if limiter.Allow(ctx) {
					mu.Lock()
					allowed++
					mu.Unlock()
				}
			})
		}

		wg.Wait()

		if allowed > 100 {
			t.Errorf("允许的请求数 %d 超过限制 100", allowed)
		}
	})
}

func TestFixedWindow(t *testing.T) {
	t.Run("基本限流", func(t *testing.T) {
		limiter := NewFixedWindow(3, 100*time.Millisecond)
		ctx := t.Context()

		for i := 0; i < 3; i++ {
			if !limiter.Allow(ctx) {
				t.Errorf("第 %d 个请求应该通过", i+1)
			}
		}
		if limiter.Allow(ctx) {
			t.Error("第4个请求应该被限流")
		}
	})

	t.Run("窗口重置", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			limiter := NewFixedWindow(2, 100*time.Millisecond)
			ctx := t.Context()

			limiter.Allow(ctx)
			limiter.Allow(ctx)

			// synctest 虚拟时钟：立即推进，无需真实等待
			time.Sleep(150 * time.Millisecond)

			if !limiter.Allow(ctx) {
				t.Error("窗口重置后应该允许请求")
			}
		})
	})
}

func TestConfig(t *testing.T) {
	t.Run("TokenBucket配置", func(t *testing.T) {
		cfg := &Config{
			Algorithm: AlgorithmTokenBucket,
			Rate:      10,
			Capacity:  5,
		}

		limiter, err := NewLimiter(cfg)
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		ctx := t.Context()
		for i := 0; i < 5; i++ {
			if !limiter.Allow(ctx) {
				t.Errorf("第 %d 个请求应该通过", i+1)
			}
		}
	})

	t.Run("SlidingWindow配置", func(t *testing.T) {
		cfg := &Config{
			Algorithm: AlgorithmSlidingWindow,
			Limit:     5,
			Window:    time.Second,
		}

		limiter, err := NewLimiter(cfg)
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		ctx := t.Context()
		for i := 0; i < 5; i++ {
			if !limiter.Allow(ctx) {
				t.Errorf("第 %d 个请求应该通过", i+1)
			}
		}
	})

	t.Run("FixedWindow配置", func(t *testing.T) {
		cfg := &Config{
			Algorithm: AlgorithmFixedWindow,
			Limit:     5,
			Window:    time.Second,
		}

		limiter, err := NewLimiter(cfg)
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		ctx := t.Context()
		for i := 0; i < 5; i++ {
			if !limiter.Allow(ctx) {
				t.Errorf("第 %d 个请求应该通过", i+1)
			}
		}
	})

	t.Run("无效配置", func(t *testing.T) {
		testCases := []struct {
			name string
			cfg  *Config
		}{
			{"空算法", &Config{}},
			{"无效算法", &Config{Algorithm: "invalid"}},
			{"令牌桶无速率", &Config{Algorithm: AlgorithmTokenBucket, Capacity: 10}},
			{"令牌桶无容量", &Config{Algorithm: AlgorithmTokenBucket, Rate: 10}},
			{"滑动窗口无限制", &Config{Algorithm: AlgorithmSlidingWindow, Window: time.Second}},
			{"滑动窗口无窗口", &Config{Algorithm: AlgorithmSlidingWindow, Limit: 10}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := NewLimiter(tc.cfg)
				if err == nil {
					t.Error("期望配置验证失败")
				}
			})
		}
	})
}
