package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/v2/storage/cache"
)

// DistributedLimiter 分布式限流器.
// 使用 RateCounter 接口实现分布式限流.
type DistributedLimiter struct {
	counter  RateCounter
	prefix   string
	limit    int
	window   time.Duration
	failOpen bool
}

// DistributedConfig 分布式限流配置.
type DistributedConfig struct {
	// Counter 计数器实现（可用 CacheRateCounter 适配 cache.Cache）
	Counter RateCounter

	// Prefix 缓存键前缀
	Prefix string

	// Limit 窗口内允许的最大请求数
	Limit int

	// Window 窗口大小
	Window time.Duration

	// FailOpen 当 Redis 等后端出错时是否放行请求.
	// 默认 true（放行），保持向后兼容.
	// 设置为 false 时，后端错误将导致请求被拒绝.
	FailOpen *bool
}

// NewDistributedLimiter 创建分布式限流器.
func NewDistributedLimiter(cfg *DistributedConfig) (*DistributedLimiter, error) {
	if cfg == nil {
		return nil, ErrInvalidConfig
	}
	if cfg.Counter == nil {
		return nil, ErrNilCache
	}
	if cfg.Limit <= 0 {
		return nil, fmt.Errorf("%w: limit 必须大于 0", ErrInvalidConfig)
	}
	if cfg.Window <= 0 {
		return nil, fmt.Errorf("%w: window 必须大于 0", ErrInvalidConfig)
	}

	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "ratelimit"
	}

	failOpen := true
	if cfg.FailOpen != nil {
		failOpen = *cfg.FailOpen
	}

	return &DistributedLimiter{
		counter:  cfg.Counter,
		prefix:   prefix,
		limit:    cfg.Limit,
		window:   cfg.Window,
		failOpen: failOpen,
	}, nil
}

// Allow 检查是否允许请求通过.
func (dl *DistributedLimiter) Allow(ctx context.Context) bool {
	return dl.AllowWithKey(ctx, "default")
}

// AllowN 检查是否允许 n 个请求通过.
func (dl *DistributedLimiter) AllowN(ctx context.Context, n int) bool {
	return dl.AllowNWithKey(ctx, "default", n)
}

// AllowWithKey 检查指定键是否允许请求通过.
func (dl *DistributedLimiter) AllowWithKey(ctx context.Context, key string) bool {
	return dl.AllowNWithKey(ctx, key, 1)
}

// AllowNWithKey 检查指定键是否允许 n 个请求通过.
func (dl *DistributedLimiter) AllowNWithKey(ctx context.Context, key string, n int) bool {
	cacheKey := fmt.Sprintf("%s:%s", dl.prefix, key)

	// 使用原子递增+过期操作，避免 INCR 和 EXPIRE 之间的竞态
	count, err := dl.counter.IncrByWithExpire(ctx, cacheKey, int64(n), dl.window)
	if err != nil {
		// 发生错误时根据 failOpen 配置决定是否放行
		return dl.failOpen
	}

	return count <= int64(dl.limit)
}

// Wait 阻塞等待直到允许请求通过.
func (dl *DistributedLimiter) Wait(ctx context.Context) error {
	return dl.WaitWithKey(ctx, "default")
}

// WaitN 阻塞等待直到允许 n 个请求通过.
func (dl *DistributedLimiter) WaitN(ctx context.Context, n int) error {
	return dl.WaitNWithKey(ctx, "default", n)
}

// WaitWithKey 阻塞等待指定键直到允许请求通过.
func (dl *DistributedLimiter) WaitWithKey(ctx context.Context, key string) error {
	return dl.WaitNWithKey(ctx, key, 1)
}

// WaitNWithKey 阻塞等待指定键直到允许 n 个请求通过.
func (dl *DistributedLimiter) WaitNWithKey(ctx context.Context, key string, n int) error {
	for {
		if dl.AllowNWithKey(ctx, key, n) {
			return nil
		}

		// 获取剩余等待时间
		cacheKey := fmt.Sprintf("%s:%s", dl.prefix, key)
		ttl, err := dl.counter.TTL(ctx, cacheKey)
		if err != nil || ttl <= 0 {
			ttl = time.Millisecond * 100
		}

		timer := time.NewTimer(ttl)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			// 继续尝试
		}
	}
}

// KeyedDistributedLimiter 基于键的分布式限流器工厂.
type KeyedDistributedLimiter struct {
	counter  RateCounter
	prefix   string
	limit    int
	window   time.Duration
	failOpen bool
}

// NewKeyedDistributedLimiter 创建基于键的分布式限流器工厂.
func NewKeyedDistributedLimiter(cfg *DistributedConfig) (*KeyedDistributedLimiter, error) {
	if cfg == nil {
		return nil, ErrInvalidConfig
	}
	if cfg.Counter == nil {
		return nil, ErrNilCache
	}
	if cfg.Limit <= 0 {
		return nil, fmt.Errorf("%w: limit 必须大于 0", ErrInvalidConfig)
	}
	if cfg.Window <= 0 {
		return nil, fmt.Errorf("%w: window 必须大于 0", ErrInvalidConfig)
	}

	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "ratelimit"
	}

	failOpen := true
	if cfg.FailOpen != nil {
		failOpen = *cfg.FailOpen
	}

	return &KeyedDistributedLimiter{
		counter:  cfg.Counter,
		prefix:   prefix,
		limit:    cfg.Limit,
		window:   cfg.Window,
		failOpen: failOpen,
	}, nil
}

// GetLimiter 获取指定键的限流器.
// 返回 KeyedLimiterFunc 以便与 KeyedEndpointMiddleware 等配合使用.
func (kdl *KeyedDistributedLimiter) GetLimiter(key string) Limiter {
	return &keyedDistributedLimiterInstance{
		counter:  kdl.counter,
		key:      fmt.Sprintf("%s:%s", kdl.prefix, key),
		limit:    kdl.limit,
		window:   kdl.window,
		failOpen: kdl.failOpen,
	}
}

type keyedDistributedLimiterInstance struct {
	counter  RateCounter
	key      string
	limit    int
	window   time.Duration
	failOpen bool
}

func (i *keyedDistributedLimiterInstance) Allow(ctx context.Context) bool {
	return i.AllowN(ctx, 1)
}

func (i *keyedDistributedLimiterInstance) AllowN(ctx context.Context, n int) bool {
	// 使用原子递增+过期操作
	count, err := i.counter.IncrByWithExpire(ctx, i.key, int64(n), i.window)
	if err != nil {
		return i.failOpen
	}

	return count <= int64(i.limit)
}

func (i *keyedDistributedLimiterInstance) Wait(ctx context.Context) error {
	return i.WaitN(ctx, 1)
}

func (i *keyedDistributedLimiterInstance) WaitN(ctx context.Context, n int) error {
	for {
		if i.AllowN(ctx, n) {
			return nil
		}

		ttl, err := i.counter.TTL(ctx, i.key)
		if err != nil || ttl <= 0 {
			ttl = time.Millisecond * 100
		}

		timer := time.NewTimer(ttl)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// cacheRateCounter 是 cache.Cache 到 RateCounter 的适配器.
type cacheRateCounter struct {
	cache cache.Cache
}

// CacheRateCounter 将 cache.Cache 适配为 RateCounter 接口.
// 示例:
//
//	redisCache, _ := cache.New(&cache.Config{Type: "redis", ...})
//	counter := ratelimit.CacheRateCounter(redisCache)
//	limiter, _ := ratelimit.NewDistributedLimiter(&ratelimit.DistributedConfig{
//	    Counter: counter,
//	    Limit:   100,
//	    Window:  time.Minute,
//	})
func CacheRateCounter(c cache.Cache) RateCounter {
	return &cacheRateCounter{cache: c}
}

func (c *cacheRateCounter) IncrementBy(ctx context.Context, key string, n int64) (int64, error) {
	return c.cache.IncrementBy(ctx, key, n)
}

func (c *cacheRateCounter) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.cache.Expire(ctx, key, ttl)
}

func (c *cacheRateCounter) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.cache.TTL(ctx, key)
}

// IncrByWithExpire 原子递增并设置过期时间.
// cache.Cache 接口不提供原子 INCR+EXPIRE，此处降级为先 INCR 再 EXPIRE.
// 如需真正原子性，请使用支持 Lua 脚本的 RateCounter 实现.
func (c *cacheRateCounter) IncrByWithExpire(ctx context.Context, key string, n int64, ttl time.Duration) (int64, error) {
	count, err := c.cache.IncrementBy(ctx, key, n)
	if err != nil {
		return 0, err
	}
	// 仅首次（count == n）设置过期时间
	if count == n {
		_ = c.cache.Expire(ctx, key, ttl)
	}
	return count, nil
}
