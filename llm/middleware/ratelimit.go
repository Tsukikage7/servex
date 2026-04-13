package middleware

import (
	"context"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/middleware/ratelimit"
)

// RateLimit 返回基于 ratelimit.Limiter 的限流中间件.
// 每次 Generate/Stream 前调用 limiter.Wait，被取消时返回 context 错误.
func RateLimit(limiter ratelimit.Limiter) Middleware {
	return func(next llm.ChatModel) llm.ChatModel {
		return Wrap(
			func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
				if err := limiter.Wait(ctx); err != nil {
					return nil, err
				}
				return next.Generate(ctx, messages, opts...)
			},
			func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
				if err := limiter.Wait(ctx); err != nil {
					return nil, err
				}
				return next.Stream(ctx, messages, opts...)
			},
		)
	}
}
