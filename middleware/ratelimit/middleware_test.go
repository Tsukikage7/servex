package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestEndpointMiddleware(t *testing.T) {
	t.Run("允许请求", func(t *testing.T) {
		limiter := NewTokenBucket(10, 10)
		middleware := EndpointMiddleware(limiter)

		endpoint := middleware(func(ctx context.Context, req any) (any, error) {
			return "success", nil
		})

		resp, err := endpoint(t.Context(), nil)
		if err != nil {
			t.Errorf("不期望错误: %v", err)
		}
		if resp != "success" {
			t.Error("期望返回 success")
		}
	})

	t.Run("拒绝请求", func(t *testing.T) {
		limiter := NewTokenBucket(1, 1)
		middleware := EndpointMiddleware(limiter)

		endpoint := middleware(func(ctx context.Context, req any) (any, error) {
			return "success", nil
		})

		// 第一个请求通过
		_, _ = endpoint(t.Context(), nil)

		// 第二个请求被限流
		_, err := endpoint(t.Context(), nil)
		if !errors.Is(err, ErrRateLimited) {
			t.Errorf("期望 ErrRateLimited，得到 %v", err)
		}
	})
}

func TestEndpointMiddlewareWithWait(t *testing.T) {
	t.Run("等待后通过", func(t *testing.T) {
		limiter := NewTokenBucket(100, 1) // 每10ms补充1个令牌
		middleware := EndpointMiddlewareWithWait(limiter)

		endpoint := middleware(func(ctx context.Context, req any) (any, error) {
			return "success", nil
		})

		// 消耗令牌
		endpoint(t.Context(), nil)

		// 应该等待后通过
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()

		resp, err := endpoint(ctx, nil)
		if err != nil {
			t.Errorf("不期望错误: %v", err)
		}
		if resp != "success" {
			t.Error("期望返回 success")
		}
	})

	t.Run("超时", func(t *testing.T) {
		limiter := NewTokenBucket(0.1, 1) // 每10秒补充1个令牌
		middleware := EndpointMiddlewareWithWait(limiter)

		endpoint := middleware(func(ctx context.Context, req any) (any, error) {
			return "success", nil
		})

		// 消耗令牌
		endpoint(t.Context(), nil)

		// 应该超时
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		_, err := endpoint(ctx, nil)
		if err != context.DeadlineExceeded {
			t.Errorf("期望 DeadlineExceeded，得到 %v", err)
		}
	})
}

func TestKeyedEndpointMiddleware(t *testing.T) {
	t.Run("不同键独立限流", func(t *testing.T) {
		limiters := make(map[string]Limiter)
		limiters["user1"] = NewTokenBucket(1, 1)
		limiters["user2"] = NewTokenBucket(1, 1)

		keyFunc := func(ctx context.Context, req any) string {
			return req.(string)
		}
		getLimiter := func(key string) Limiter {
			return limiters[key]
		}

		middleware := KeyedEndpointMiddleware(keyFunc, getLimiter)
		endpoint := middleware(func(ctx context.Context, req any) (any, error) {
			return "success", nil
		})

		ctx := t.Context()

		// user1 第一个请求通过
		_, err := endpoint(ctx, "user1")
		if err != nil {
			t.Errorf("user1 第一个请求不应该被限流: %v", err)
		}

		// user1 第二个请求被限流
		_, err = endpoint(ctx, "user1")
		if !errors.Is(err, ErrRateLimited) {
			t.Errorf("user1 第二个请求应该被限流: %v", err)
		}

		// user2 第一个请求通过独立限流
		_, err = endpoint(ctx, "user2")
		if err != nil {
			t.Errorf("user2 第一个请求不应该被限流: %v", err)
		}
	})

	t.Run("未知键放行", func(t *testing.T) {
		getLimiter := func(key string) Limiter {
			return nil // 返回 nil 表示不限流
		}

		middleware := KeyedEndpointMiddleware(
			func(ctx context.Context, req any) string { return "unknown" },
			getLimiter,
		)
		endpoint := middleware(func(ctx context.Context, req any) (any, error) {
			return "success", nil
		})

		resp, err := endpoint(t.Context(), nil)
		if err != nil {
			t.Errorf("未知键应该放行: %v", err)
		}
		if resp != "success" {
			t.Error("期望返回 success")
		}
	})
}
