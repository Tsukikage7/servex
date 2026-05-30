package semaphore

import (
	"context"
	"strconv"
	"time"

	"github.com/Tsukikage7/servex/v2/storage/cache"
)

// Distributed 分布式信号量.
// 基于 Counter 接口实现，适用于分布式并发控制.
type Distributed struct {
	counter   Counter
	key       string
	size      int64
	ttl       time.Duration
	retryWait time.Duration
}

// Option 分布式信号量配置选项.
type Option func(*Distributed)

// WithTTL 设置许可的过期时间.
// 防止因客户端崩溃导致许可无法释放.
// 默认 30 秒.
func WithTTL(ttl time.Duration) Option {
	return func(s *Distributed) {
		s.ttl = ttl
	}
}

// WithRetryWait 设置重试等待时间.
// 当无法获取许可时，等待多长时间后重试.
// 默认 100ms.
func WithRetryWait(wait time.Duration) Option {
	return func(s *Distributed) {
		s.retryWait = wait
	}
}

// New 创建分布式信号量.
// counter: 计数器实现可用 CacheCounter 适配 cache.Cache
// key: 信号量唯一标识
// size: 最大并发数
func New(counter Counter, key string, size int64, opts ...Option) *Distributed {
	if counter == nil {
		panic("semaphore: 计数器不能为空")
	}
	if size <= 0 {
		panic("semaphore: 信号量大小必须为正数")
	}

	s := &Distributed{
		counter:   counter,
		key:       "semaphore:" + key,
		size:      size,
		ttl:       30 * time.Second,
		retryWait: 100 * time.Millisecond,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Acquire 获取一个许可.
func (s *Distributed) Acquire(ctx context.Context) error {
	for {
		if s.TryAcquire(ctx) {
			return nil
		}

		retryTimer := time.NewTimer(s.retryWait)
		select {
		case <-ctx.Done():
			retryTimer.Stop()
			return ctx.Err()
		case <-retryTimer.C:
			// 重试
		}
	}
}

// TryAcquire 尝试获取一个许可.
func (s *Distributed) TryAcquire(ctx context.Context) bool {
	// 使用原子增加计数
	count, err := s.counter.Increment(ctx, s.key)
	if err != nil {
		return false
	}

	// 首次创建时设置过期时间
	if count == 1 {
		_ = s.counter.Expire(ctx, s.key, s.ttl)
	}

	// 检查是否超过限制
	if count > s.size {
		// 超过限制，回退
		_, _ = s.counter.Decrement(ctx, s.key)
		return false
	}

	// 刷新过期时间
	_ = s.counter.Expire(ctx, s.key, s.ttl)
	return true
}

// Release 释放一个许可.
func (s *Distributed) Release(ctx context.Context) error {
	_, err := s.counter.Decrement(ctx, s.key)
	return err
}

// Available 返回当前可用的许可数量.
func (s *Distributed) Available(ctx context.Context) (int64, error) {
	current, err := s.counter.Get(ctx, s.key)
	if err != nil {
		// 键不存在，返回全部可用
		return s.size, nil
	}

	available := s.size - current
	if available < 0 {
		available = 0
	}
	return available, nil
}

// Size 返回信号量的总大小.
func (s *Distributed) Size() int64 {
	return s.size
}

// cacheCounter 是 cache.Cache 到 Counter 的适配器.
type cacheCounter struct {
	cache cache.Cache
}

// CacheCounter 将 cache.Cache 适配为 Counter 接口.
// 示例:
//
//	redisCache, _ := cache.New(&cache.Config{Type: "redis", ...})
//	counter := semaphore.CacheCounter(redisCache)
//	sem := semaphore.New(counter, "api-limit", 100)
func CacheCounter(c cache.Cache) Counter {
	return &cacheCounter{cache: c}
}

func (c *cacheCounter) Increment(ctx context.Context, key string) (int64, error) {
	return c.cache.Increment(ctx, key)
}

func (c *cacheCounter) Decrement(ctx context.Context, key string) (int64, error) {
	return c.cache.Decrement(ctx, key)
}

func (c *cacheCounter) Get(ctx context.Context, key string) (int64, error) {
	val, err := c.cache.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	if val == "" {
		return 0, nil
	}
	return strconv.ParseInt(val, 10, 64)
}

func (c *cacheCounter) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.cache.Expire(ctx, key, ttl)
}
