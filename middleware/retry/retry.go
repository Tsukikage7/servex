// Package retry 提供重试机制.
package retry

import (
	"context"
	"time"
)

const (
	// DefaultMaxAttempts 默认最大重试次数.
	DefaultMaxAttempts = 3
	// DefaultDelay 默认重试延迟.
	DefaultDelay = 100 * time.Millisecond
)

// Retry 重试器.
type Retry struct {
	ctx         context.Context
	fn          func() error
	maxAttempts int
	delay       time.Duration
}

// Do 创建重试器.
// 使用示例:
//	err := retry.Do(ctx, func() error {
//	    return someOperation()
//	}).Run()
//	err := retry.Do(ctx, func() error {
//	    return someOperation()
//	}).WithMaxAttempts(5).WithDelay(time.Second).Run()
func Do(ctx context.Context, fn func() error) *Retry {
	return &Retry{
		ctx:         ctx,
		fn:          fn,
		maxAttempts: DefaultMaxAttempts,
		delay:       DefaultDelay,
	}
}

// WithMaxAttempts 设置最大重试次数.
func (r *Retry) WithMaxAttempts(n int) *Retry {
	r.maxAttempts = n
	return r
}

// WithDelay 设置重试间隔.
func (r *Retry) WithDelay(d time.Duration) *Retry {
	r.delay = d
	return r
}

// Run 执行重试.
func (r *Retry) Run() error {
	for attempt := 0; attempt < r.maxAttempts; attempt++ {
		// 检查上下文是否已取消
		select {
		case <-r.ctx.Done():
			return r.ctx.Err()
		default:
		}

		// 执行函数
		if err := r.fn(); err == nil {
			return nil
		}

		// 如果不是最后一次尝试，则等待重试延迟
		if attempt < r.maxAttempts-1 {
			timer := time.NewTimer(r.delay)
			select {
			case <-timer.C:
				continue
			case <-r.ctx.Done():
				timer.Stop()
				return r.ctx.Err()
			}
		}
	}

	return ErrMaxAttempts
}
