package ratelimit

import (
	"context"

	"github.com/Tsukikage7/servex/v2/endpoint"
)

// EndpointMiddleware 创建限流 Endpoint 中间件.
// 当请求被限流时返回 ErrRateLimited 错误.
func EndpointMiddleware(limiter Limiter) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (any, error) {
			if !limiter.Allow(ctx) {
				return nil, ErrRateLimited
			}
			return next(ctx, request)
		}
	}
}

// EndpointMiddlewareWithWait 创建阻塞式限流 Endpoint 中间件.
// 当请求被限流时阻塞等待，直到可以通过或 context 超时.
func EndpointMiddlewareWithWait(limiter Limiter) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (any, error) {
			if err := limiter.Wait(ctx); err != nil {
				return nil, err
			}
			return next(ctx, request)
		}
	}
}

// KeyFunc 用于提取限流键的函数类型.
// 可以基于用户 ID、IP 地址等进行限流.
type KeyFunc func(ctx context.Context, request any) string

// KeyedLimiterFunc 用于获取指定键的限流器.
type KeyedLimiterFunc func(key string) Limiter

// KeyedEndpointMiddleware 创建基于键的限流 Endpoint 中间件.
// 可以为不同的用户/IP 使用不同的限流策略.
func KeyedEndpointMiddleware(keyFunc KeyFunc, getLimiter KeyedLimiterFunc) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (any, error) {
			key := keyFunc(ctx, request)
			limiter := getLimiter(key)
			if limiter == nil {
				return next(ctx, request)
			}
			if !limiter.Allow(ctx) {
				return nil, ErrRateLimited
			}
			return next(ctx, request)
		}
	}
}
