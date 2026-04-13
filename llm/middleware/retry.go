package middleware

import (
	"context"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/middleware/retry"
)

// Retry 返回对 429/5xx 错误进行指数退避重试的中间件.
//
// maxAttempts: 最大尝试次数（含首次），最少为 1.
// baseDelay: 首次重试等待时间，后续按指数退避，最大 30 秒.
// 底层复用 middleware/retry.ExponentialStrategy，带 full jitter.
func Retry(maxAttempts int, baseDelay time.Duration) Middleware {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	const maxDelay = 30 * time.Second
	return func(next llm.ChatModel) llm.ChatModel {
		return Wrap(
			func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
				strategy := retry.NewExponentialStrategy(baseDelay, maxDelay, maxAttempts)
				var lastErr error
				for attempt := range maxAttempts {
					resp, err := next.Generate(ctx, messages, opts...)
					if err == nil {
						return resp, nil
					}
					lastErr = err
					if !llm.IsRetryable(err) {
						return nil, err
					}
					if attempt == maxAttempts-1 {
						break
					}
					strategy = strategy.Report(err)
					delay, ok := strategy.Next()
					if !ok {
						break
					}
					retryTimer := time.NewTimer(delay)
					select {
					case <-ctx.Done():
						retryTimer.Stop()
						return nil, ctx.Err()
					case <-retryTimer.C:
					}
				}
				return nil, lastErr
			},
			func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
				strategy := retry.NewExponentialStrategy(baseDelay, maxDelay, maxAttempts)
				var lastErr error
				for attempt := range maxAttempts {
					reader, err := next.Stream(ctx, messages, opts...)
					if err == nil {
						return reader, nil
					}
					lastErr = err
					if !llm.IsRetryable(err) {
						return nil, err
					}
					if attempt == maxAttempts-1 {
						break
					}
					strategy = strategy.Report(err)
					delay, ok := strategy.Next()
					if !ok {
						break
					}
					retryTimer := time.NewTimer(delay)
					select {
					case <-ctx.Done():
						retryTimer.Stop()
						return nil, ctx.Err()
					case <-retryTimer.C:
					}
				}
				return nil, lastErr
			},
		)
	}
}
