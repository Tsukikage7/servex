// Package ratelimit 提供限流功能.
package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Limiter 限流器接口.
type Limiter interface {
	// Allow 检查是否允许请求通过.
	// 返回 true 表示允许，false 表示被限流.
	Allow(ctx context.Context) bool

	// AllowN 检查是否允许 n 个请求通过.
	AllowN(ctx context.Context, n int) bool

	// Wait 阻塞等待直到允许请求通过.
	// 返回 error 表示 context 被取消或超时.
	Wait(ctx context.Context) error

	// WaitN 阻塞等待直到允许 n 个请求通过.
	WaitN(ctx context.Context, n int) error
}

// RateCounter 分布式限流器所需的计数器接口.
// 这是分布式限流的最小依赖接口.
// 可以用 cache.Cache、Redis 客户端或其他存储实现.
type RateCounter interface {
	// IncrementBy 原子增加计数并返回新值.
	IncrementBy(ctx context.Context, key string, n int64) (int64, error)

	// Expire 设置键的过期时间.
	Expire(ctx context.Context, key string, ttl time.Duration) error

	// TTL 获取键的剩余过期时间.
	TTL(ctx context.Context, key string) (time.Duration, error)

	// IncrByWithExpire 原子增加计数并设置过期时间，返回新值.
	// 实现应保证 INCR 和 EXPIRE 的原子性（如使用 Lua 脚本）.
	// 如果底层不支持原子操作，可降级为先 IncrementBy 再 Expire.
	IncrByWithExpire(ctx context.Context, key string, n int64, ttl time.Duration) (int64, error)
}

// TokenBucket 令牌桶限流器.
// 令牌以固定速率生成，请求需要消耗令牌才能通过.
// 适合平滑突发流量.
type TokenBucket struct {
	mu sync.Mutex

	rate       float64   // 每秒生成的令牌数
	capacity   float64   // 桶容量
	tokens     float64   // 当前令牌数
	lastUpdate time.Time // 上次更新时间
}

// NewTokenBucket 创建令牌桶限流器.
// rate: 每秒生成的令牌数
// capacity: 桶容量（最大令牌数）
func NewTokenBucket(rate float64, capacity float64) *TokenBucket {
	return &TokenBucket{
		rate:       rate,
		capacity:   capacity,
		tokens:     capacity, // 初始满桶
		lastUpdate: time.Now(),
	}
}

// Allow 检查是否允许一个请求通过.
func (tb *TokenBucket) Allow(ctx context.Context) bool {
	return tb.AllowN(ctx, 1)
}

// AllowN 检查是否允许 n 个请求通过.
func (tb *TokenBucket) AllowN(_ context.Context, n int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens >= float64(n) {
		tb.tokens -= float64(n)
		return true
	}
	return false
}

// Wait 阻塞等待直到允许请求通过.
func (tb *TokenBucket) Wait(ctx context.Context) error {
	return tb.WaitN(ctx, 1)
}

// WaitN 阻塞等待直到允许 n 个请求通过.
func (tb *TokenBucket) WaitN(ctx context.Context, n int) error {
	for {
		if tb.AllowN(ctx, n) {
			return nil
		}

		// 计算需要等待的时间
		tb.mu.Lock()
		tokensNeeded := float64(n) - tb.tokens
		waitTime := time.Duration(tokensNeeded / tb.rate * float64(time.Second))
		tb.mu.Unlock()

		// 最少等待 1ms
		if waitTime < time.Millisecond {
			waitTime = time.Millisecond
		}

		timer := time.NewTimer(waitTime)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			// 继续尝试
		}
	}
}

// refill 补充令牌（需要持有锁）.
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastUpdate).Seconds()
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastUpdate = now
}

// SlidingWindow 滑动窗口限流器.
// 统计最近一个时间窗口内的请求数，超过阈值则拒绝.
// 适合精确控制 QPS.
type SlidingWindow struct {
	mu sync.Mutex

	limit      int           // 窗口内允许的最大请求数
	window     time.Duration // 窗口大小
	timestamps []time.Time   // 请求时间戳
}

// NewSlidingWindow 创建滑动窗口限流器.
// limit: 窗口内允许的最大请求数
// window: 窗口大小
func NewSlidingWindow(limit int, window time.Duration) *SlidingWindow {
	return &SlidingWindow{
		limit:      limit,
		window:     window,
		timestamps: make([]time.Time, 0, limit),
	}
}

// Allow 检查是否允许一个请求通过.
func (sw *SlidingWindow) Allow(ctx context.Context) bool {
	return sw.AllowN(ctx, 1)
}

// AllowN 检查是否允许 n 个请求通过.
func (sw *SlidingWindow) AllowN(_ context.Context, n int) bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	sw.cleanup(now)

	if len(sw.timestamps)+n <= sw.limit {
		for range n {
			sw.timestamps = append(sw.timestamps, now)
		}
		return true
	}
	return false
}

// Wait 阻塞等待直到允许请求通过.
func (sw *SlidingWindow) Wait(ctx context.Context) error {
	return sw.WaitN(ctx, 1)
}

// WaitN 阻塞等待直到允许 n 个请求通过.
func (sw *SlidingWindow) WaitN(ctx context.Context, n int) error {
	for {
		if sw.AllowN(ctx, n) {
			return nil
		}

		// 计算需要等待的时间
		sw.mu.Lock()
		var waitTime time.Duration
		if len(sw.timestamps) > 0 {
			oldest := sw.timestamps[0]
			waitTime = sw.window - time.Since(oldest)
		} else {
			waitTime = time.Millisecond
		}
		sw.mu.Unlock()

		if waitTime < time.Millisecond {
			waitTime = time.Millisecond
		}

		timer := time.NewTimer(waitTime)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			// 继续尝试
		}
	}
}

// cleanup 清理过期的时间戳（需要持有锁）.
func (sw *SlidingWindow) cleanup(now time.Time) {
	cutoff := now.Add(-sw.window)
	i := 0
	for i < len(sw.timestamps) && sw.timestamps[i].Before(cutoff) {
		i++
	}
	if i > 0 {
		sw.timestamps = sw.timestamps[i:]
		// 当底层数组容量远大于实际长度时，复制到新切片以释放旧内存
		if cap(sw.timestamps) > 2*len(sw.timestamps)+1 {
			trimmed := make([]time.Time, len(sw.timestamps))
			copy(trimmed, sw.timestamps)
			sw.timestamps = trimmed
		}
	}
}

// FixedWindow 固定窗口限流器.
// 将时间划分为固定窗口，每个窗口内限制请求数.
// 实现简单，但可能有边界突发问题.
type FixedWindow struct {
	mu sync.Mutex

	limit       int           // 窗口内允许的最大请求数
	window      time.Duration // 窗口大小
	count       int           // 当前窗口请求数
	windowStart time.Time     // 当前窗口开始时间
}

// NewFixedWindow 创建固定窗口限流器.
func NewFixedWindow(limit int, window time.Duration) *FixedWindow {
	return &FixedWindow{
		limit:       limit,
		window:      window,
		windowStart: time.Now(),
	}
}

// Allow 检查是否允许一个请求通过.
func (fw *FixedWindow) Allow(ctx context.Context) bool {
	return fw.AllowN(ctx, 1)
}

// AllowN 检查是否允许 n 个请求通过.
func (fw *FixedWindow) AllowN(_ context.Context, n int) bool {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	now := time.Now()
	if now.Sub(fw.windowStart) >= fw.window {
		fw.windowStart = now
		fw.count = 0
	}

	if fw.count+n <= fw.limit {
		fw.count += n
		return true
	}
	return false
}

// Wait 阻塞等待直到允许请求通过.
func (fw *FixedWindow) Wait(ctx context.Context) error {
	return fw.WaitN(ctx, 1)
}

// WaitN 阻塞等待直到允许 n 个请求通过.
func (fw *FixedWindow) WaitN(ctx context.Context, n int) error {
	for {
		if fw.AllowN(ctx, n) {
			return nil
		}

		// 计算需要等待的时间
		fw.mu.Lock()
		waitTime := fw.window - time.Since(fw.windowStart)
		fw.mu.Unlock()

		if waitTime < time.Millisecond {
			waitTime = time.Millisecond
		}

		timer := time.NewTimer(waitTime)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			// 继续尝试
		}
	}
}
