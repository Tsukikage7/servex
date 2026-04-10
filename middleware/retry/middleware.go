package retry

import (
	"context"
	"time"

	"github.com/Tsukikage7/servex/endpoint"
)

// Config 重试配置.
type Config struct {
	MaxAttempts int           // 最大重试次数
	Delay       time.Duration // 重试间隔
	Backoff     BackoffFunc   // 退避策略
	Retryable   RetryableFunc // 判断是否应该重试
}

// BackoffFunc 计算第 n 次重试的等待时间.
type BackoffFunc func(attempt int, delay time.Duration) time.Duration

// RetryableFunc 判断错误是否应该重试.
type RetryableFunc func(err error) bool

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		MaxAttempts: DefaultMaxAttempts,
		Delay:       DefaultDelay,
		Backoff:     FixedBackoff,
		Retryable:   AlwaysRetry,
	}
}

// FixedBackoff 固定退避策略.
func FixedBackoff(_ int, delay time.Duration) time.Duration {
	return delay
}

// ExponentialBackoff 指数退避策略.
func ExponentialBackoff(attempt int, delay time.Duration) time.Duration {
	return delay * time.Duration(1<<uint(attempt))
}

// LinearBackoff 线性退避策略.
func LinearBackoff(attempt int, delay time.Duration) time.Duration {
	return delay * time.Duration(attempt+1)
}

// AlwaysRetry 总是重试.
func AlwaysRetry(_ error) bool {
	return true
}

// NeverRetry 从不重试.
func NeverRetry(_ error) bool {
	return false
}

// EndpointMiddleware 返回 Endpoint 重试中间件.
// 使用示例:
//
//	cfg := retry.DefaultConfig()
//	endpoint = retry.EndpointMiddleware(cfg)(endpoint)
func EndpointMiddleware(cfg *Config) endpoint.Middleware {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.Backoff == nil {
		cfg.Backoff = FixedBackoff
	}
	if cfg.Retryable == nil {
		cfg.Retryable = AlwaysRetry
	}

	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (response any, err error) {
			for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
				// 检查上下文是否已取消
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
				}

				// 执行 endpoint
				response, err = next(ctx, request)
				if err == nil {
					return response, nil
				}

				// 判断是否应该重试
				if !cfg.Retryable(err) {
					return response, err
				}

				// 如果不是最后一次尝试，则等待
				if attempt < cfg.MaxAttempts-1 {
					wait := cfg.Backoff(attempt, cfg.Delay)
					select {
					case <-time.After(wait):
						continue
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				}
			}

			return response, err
		}
	}
}

// EndpointRetrier 提供可配置的 Endpoint 重试器.
type EndpointRetrier struct {
	cfg *Config
}

// NewEndpointRetrier 创建 Endpoint 重试器.
func NewEndpointRetrier(cfg *Config) *EndpointRetrier {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &EndpointRetrier{cfg: cfg}
}

// Middleware 返回重试中间件.
func (r *EndpointRetrier) Middleware() endpoint.Middleware {
	return EndpointMiddleware(r.cfg)
}

// WithMaxAttempts 设置最大重试次数.
func (r *EndpointRetrier) WithMaxAttempts(n int) *EndpointRetrier {
	r.cfg.MaxAttempts = n
	return r
}

// WithDelay 设置重试间隔.
func (r *EndpointRetrier) WithDelay(d time.Duration) *EndpointRetrier {
	r.cfg.Delay = d
	return r
}

// WithBackoff 设置退避策略.
func (r *EndpointRetrier) WithBackoff(fn BackoffFunc) *EndpointRetrier {
	r.cfg.Backoff = fn
	return r
}

// WithRetryable 设置重试判断函数.
func (r *EndpointRetrier) WithRetryable(fn RetryableFunc) *EndpointRetrier {
	r.cfg.Retryable = fn
	return r
}
