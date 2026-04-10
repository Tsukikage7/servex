// Package counter 提供分布式计数器实现.
// 区别于 storage/redis 的原子操作，本包提供滑动窗口统计、批量聚合等
// 业务级计数能力.
// 基本用法:
//
//	// 内存实现（适合测试或单进程场景）
//	c := counter.NewMemoryCounter(counter.WithPrefix("app:"))
//	// Redis 实现（分布式场景）
//	c := counter.NewRedisCounter(redisClient, counter.WithPrefix("app:"))
//	// 简单计数
//	val, _ := c.Incr(ctx, "login_count", 1)
//	// 滑动窗口计数
//	val, _ = c.IncrWindow(ctx, "api_calls", 5*time.Minute)
//	count, _ := c.GetWindow(ctx, "api_calls", 5*time.Minute)
package counter

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Counter 分布式计数器接口.
type Counter interface {
	// Incr 增加计数，返回增加后的值.
	Incr(ctx context.Context, key string, delta int64) (int64, error)
	// Get 获取当前计数.
	Get(ctx context.Context, key string) (int64, error)
	// Reset 重置计数.
	Reset(ctx context.Context, key string) error

	// IncrWindow 滑动窗口计数（最近 N 时间内的计数）.
	IncrWindow(ctx context.Context, key string, window time.Duration) (int64, error)
	// GetWindow 获取滑动窗口内的计数.
	GetWindow(ctx context.Context, key string, window time.Duration) (int64, error)

	// MGet 批量获取计数.
	MGet(ctx context.Context, keys ...string) (map[string]int64, error)
}

// options 计数器选项.
type options struct {
	prefix string
}

// Option 计数器选项函数.
type Option func(*options)

// WithPrefix 设置键前缀.
func WithPrefix(prefix string) Option {
	return func(o *options) {
		o.prefix = prefix
	}
}

func applyOptions(opts []Option) *options {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// ---- 内存实现 ----

// memoryCounter 基于内存的计数器实现（单进程，用于测试）.
type memoryCounter struct {
	mu      sync.RWMutex
	counts  map[string]int64
	windows map[string][]time.Time // key -> 时间戳列表
	opts    *options
}

// NewMemoryCounter 创建内存计数器.
func NewMemoryCounter(opts ...Option) Counter {
	return &memoryCounter{
		counts:  make(map[string]int64),
		windows: make(map[string][]time.Time),
		opts:    applyOptions(opts),
	}
}

func (c *memoryCounter) fullKey(key string) string {
	return c.opts.prefix + key
}

func (c *memoryCounter) Incr(_ context.Context, key string, delta int64) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	k := c.fullKey(key)
	c.counts[k] += delta
	return c.counts[k], nil
}

func (c *memoryCounter) Get(_ context.Context, key string) (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.counts[c.fullKey(key)], nil
}

func (c *memoryCounter) Reset(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	k := c.fullKey(key)
	delete(c.counts, k)
	delete(c.windows, k)
	return nil
}

func (c *memoryCounter) IncrWindow(_ context.Context, key string, window time.Duration) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	k := c.fullKey(key)
	now := time.Now()
	cutoff := now.Add(-window)

	// 清理过期记录
	c.windows[k] = filterAfter(c.windows[k], cutoff)
	// 添加新记录
	c.windows[k] = append(c.windows[k], now)
	return int64(len(c.windows[k])), nil
}

func (c *memoryCounter) GetWindow(_ context.Context, key string, window time.Duration) (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	k := c.fullKey(key)
	cutoff := time.Now().Add(-window)
	count := int64(0)
	for _, t := range c.windows[k] {
		if !t.Before(cutoff) {
			count++
		}
	}
	return count, nil
}

func (c *memoryCounter) MGet(_ context.Context, keys ...string) (map[string]int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]int64, len(keys))
	for _, key := range keys {
		result[key] = c.counts[c.fullKey(key)]
	}
	return result, nil
}

// filterAfter 返回在 cutoff 之后（含）的时间戳.
// 当有效元素不足底层数组容量一半时，重新分配切片以释放内存.
func filterAfter(times []time.Time, cutoff time.Time) []time.Time {
	n := 0
	for _, t := range times {
		if !t.Before(cutoff) {
			times[n] = t
			n++
		}
	}
	times = times[:n]
	// 有效元素不足底层容量一半时，拷贝到新切片释放旧数组
	if n > 0 && n < cap(times)/2 {
		shrunk := make([]time.Time, n)
		copy(shrunk, times)
		return shrunk
	}
	return times
}

// ---- Redis 实现 ----

// redisCounter 基于 Redis 的分布式计数器实现.
type redisCounter struct {
	client redis.Cmdable
	opts   *options
}

// NewRedisCounter 创建 Redis 计数器.
func NewRedisCounter(client redis.Cmdable, opts ...Option) Counter {
	return &redisCounter{
		client: client,
		opts:   applyOptions(opts),
	}
}

func (c *redisCounter) fullKey(key string) string {
	return c.opts.prefix + key
}

func (c *redisCounter) windowKey(key string) string {
	return c.opts.prefix + key + ":window"
}

func (c *redisCounter) Incr(ctx context.Context, key string, delta int64) (int64, error) {
	return c.client.IncrBy(ctx, c.fullKey(key), delta).Result()
}

func (c *redisCounter) Get(ctx context.Context, key string) (int64, error) {
	val, err := c.client.Get(ctx, c.fullKey(key)).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(val, 10, 64)
}

func (c *redisCounter) Reset(ctx context.Context, key string) error {
	pipe := c.client.Pipeline()
	pipe.Del(ctx, c.fullKey(key))
	pipe.Del(ctx, c.windowKey(key))
	_, err := pipe.Exec(ctx)
	return err
}

func (c *redisCounter) IncrWindow(ctx context.Context, key string, window time.Duration) (int64, error) {
	k := c.windowKey(key)
	now := time.Now()
	score := float64(now.UnixNano())
	member := fmt.Sprintf("%d", now.UnixNano())
	cutoff := float64(now.Add(-window).UnixNano())

	pipe := c.client.Pipeline()
	pipe.ZAdd(ctx, k, redis.Z{Score: score, Member: member})
	pipe.ZRemRangeByScore(ctx, k, "-inf", strconv.FormatFloat(cutoff, 'f', 0, 64))
	countCmd := pipe.ZCard(ctx, k)
	pipe.Expire(ctx, k, window+time.Minute) // 多保留一分钟防止边界问题
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return countCmd.Val(), nil
}

func (c *redisCounter) GetWindow(ctx context.Context, key string, window time.Duration) (int64, error) {
	k := c.windowKey(key)
	cutoff := float64(time.Now().Add(-window).UnixNano())
	return c.client.ZCount(ctx, k, strconv.FormatFloat(cutoff, 'f', 0, 64), "+inf").Result()
}

func (c *redisCounter) MGet(ctx context.Context, keys ...string) (map[string]int64, error) {
	if len(keys) == 0 {
		return make(map[string]int64), nil
	}

	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = c.fullKey(key)
	}

	vals, err := c.client.MGet(ctx, fullKeys...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string]int64, len(keys))
	for i, key := range keys {
		if vals[i] == nil {
			result[key] = 0
			continue
		}
		s, ok := vals[i].(string)
		if !ok {
			// Redis 返回了非 string 类型，跳过
			result[key] = 0
			continue
		}
		v, _ := strconv.ParseInt(s, 10, 64)
		result[key] = v
	}
	return result, nil
}
