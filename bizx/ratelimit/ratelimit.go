// Package ratelimit 提供业务级限流实现.
// 区别于 middleware/ratelimit 的请求级限流，本包提供按用户/租户的配额管理，
// 支持查看使用量、剩余配额、重置时间等信息.
// 基本用法:
//	mgr := ratelimit.NewMemoryQuotaManager()
//	quota := ratelimit.Quota{
//	    Key:    "user:123",
//	    Limit:  1000,
//	    Window: 24 * time.Hour,
//	}
//	usage, err := mgr.Consume(ctx, quota, 1)
//	if errors.Is(err, ratelimit.ErrQuotaExceeded) {
//	    // 配额已用尽
//	}
package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrQuotaExceeded 配额超限.
	ErrQuotaExceeded = errors.New("ratelimit: quota exceeded")
)

// Quota 配额定义.
type Quota struct {
	Key    string        // 配额键（如 userID, tenantID）
	Limit  int64         // 配额上限
	Window time.Duration // 配额窗口（如 24h, 720h=30天）
}

// Usage 配额使用情况.
type Usage struct {
	Used      int64     `json:"used"`
	Remaining int64     `json:"remaining"`
	Limit     int64     `json:"limit"`
	ResetsAt  time.Time `json:"resets_at"`
}

// QuotaManager 配额管理器接口.
type QuotaManager interface {
	// Check 检查配额（不消耗）.
	Check(ctx context.Context, quota Quota) (*Usage, error)
	// Consume 消耗配额.
	Consume(ctx context.Context, quota Quota, n int64) (*Usage, error)
	// Reset 重置配额.
	Reset(ctx context.Context, key string) error
	// GetUsage 获取使用量.
	GetUsage(ctx context.Context, quota Quota) (*Usage, error)
}

// options 配额管理器选项.
type options struct {
	keyPrefix string
}

// Option 配额管理器选项函数.
type Option func(*options)

// WithKeyPrefix 设置键前缀.
func WithKeyPrefix(prefix string) Option {
	return func(o *options) {
		o.keyPrefix = prefix
	}
}

func applyOptions(opts []Option) *options {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// windowStart 计算当前窗口的起始时间.
func windowStart(window time.Duration) time.Time {
	now := time.Now()
	return now.Truncate(window)
}

// windowEnd 计算当前窗口的结束时间.
func windowEnd(window time.Duration) time.Time {
	return windowStart(window).Add(window)
}

// ---- 内存实现 ----

type memQuotaEntry struct {
	used      int64
	windowKey string
}

// memoryQuotaManager 基于内存的配额管理器.
type memoryQuotaManager struct {
	mu      sync.Mutex
	entries map[string]*memQuotaEntry
}

// NewMemoryQuotaManager 创建内存配额管理器.
func NewMemoryQuotaManager() QuotaManager {
	return &memoryQuotaManager{
		entries: make(map[string]*memQuotaEntry),
	}
}

func (m *memoryQuotaManager) windowKey(quota Quota) string {
	ws := windowStart(quota.Window)
	return fmt.Sprintf("%s:%d", quota.Key, ws.UnixNano())
}

func (m *memoryQuotaManager) getOrCreate(quota Quota) *memQuotaEntry {
	wk := m.windowKey(quota)
	entry, ok := m.entries[quota.Key]
	if !ok || entry.windowKey != wk {
		entry = &memQuotaEntry{windowKey: wk}
		m.entries[quota.Key] = entry
	}
	return entry
}

func (m *memoryQuotaManager) buildUsage(entry *memQuotaEntry, quota Quota) *Usage {
	remaining := quota.Limit - entry.used
	if remaining < 0 {
		remaining = 0
	}
	return &Usage{
		Used:      entry.used,
		Remaining: remaining,
		Limit:     quota.Limit,
		ResetsAt:  windowEnd(quota.Window),
	}
}

func (m *memoryQuotaManager) Check(_ context.Context, quota Quota) (*Usage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry := m.getOrCreate(quota)
	return m.buildUsage(entry, quota), nil
}

func (m *memoryQuotaManager) Consume(_ context.Context, quota Quota, n int64) (*Usage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry := m.getOrCreate(quota)

	if entry.used+n > quota.Limit {
		return m.buildUsage(entry, quota), ErrQuotaExceeded
	}

	entry.used += n
	return m.buildUsage(entry, quota), nil
}

func (m *memoryQuotaManager) Reset(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, key)
	return nil
}

func (m *memoryQuotaManager) GetUsage(_ context.Context, quota Quota) (*Usage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry := m.getOrCreate(quota)
	return m.buildUsage(entry, quota), nil
}

// ---- Redis 实现 ----

// redisQuotaManager 基于 Redis 的配额管理器.
type redisQuotaManager struct {
	client redis.Cmdable
	opts   *options
}

// NewRedisQuotaManager 创建 Redis 配额管理器.
func NewRedisQuotaManager(client redis.Cmdable, opts ...Option) QuotaManager {
	return &redisQuotaManager{
		client: client,
		opts:   applyOptions(opts),
	}
}

func (m *redisQuotaManager) redisKey(quota Quota) string {
	ws := windowStart(quota.Window)
	return fmt.Sprintf("%squota:%s:%d", m.opts.keyPrefix, quota.Key, ws.Unix())
}

func (m *redisQuotaManager) Check(ctx context.Context, quota Quota) (*Usage, error) {
	key := m.redisKey(quota)
	val, err := m.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return &Usage{
			Used:      0,
			Remaining: quota.Limit,
			Limit:     quota.Limit,
			ResetsAt:  windowEnd(quota.Window),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	used, _ := strconv.ParseInt(val, 10, 64)
	remaining := quota.Limit - used
	if remaining < 0 {
		remaining = 0
	}
	return &Usage{
		Used:      used,
		Remaining: remaining,
		Limit:     quota.Limit,
		ResetsAt:  windowEnd(quota.Window),
	}, nil
}

// consumeScript 原子检查并增加配额的 Lua 脚本.
// KEYS[1]: 配额键
// ARGV[1]: 增加量 n
// ARGV[2]: 配额上限 limit
// ARGV[3]: TTL 秒数
// 返回: {new_value, 0} 成功, {current_value, 1} 超限
var consumeScript = redis.NewScript(`
local current = tonumber(redis.call("GET", KEYS[1]) or "0")
local n = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])
if current + n > limit then
    return {current, 1}
end
local newval = redis.call("INCRBY", KEYS[1], n)
if ttl > 0 then
    redis.call("EXPIRE", KEYS[1], ttl)
end
return {newval, 0}
`)

func (m *redisQuotaManager) Consume(ctx context.Context, quota Quota, n int64) (*Usage, error) {
	key := m.redisKey(quota)
	ttl := int64(time.Until(windowEnd(quota.Window)).Seconds())
	if ttl < 1 {
		ttl = 1
	}

	result, err := consumeScript.Run(ctx, m.client, []string{key},
		n, quota.Limit, ttl,
	).Int64Slice()
	if err != nil {
		return nil, err
	}

	used := result[0]
	exceeded := result[1] == 1

	remaining := quota.Limit - used
	if remaining < 0 {
		remaining = 0
	}
	usage := &Usage{
		Used:      used,
		Remaining: remaining,
		Limit:     quota.Limit,
		ResetsAt:  windowEnd(quota.Window),
	}
	if exceeded {
		return usage, ErrQuotaExceeded
	}
	return usage, nil
}

func (m *redisQuotaManager) Reset(ctx context.Context, key string) error {
	// 删除所有匹配的键
	pattern := fmt.Sprintf("%squota:%s:*", m.opts.keyPrefix, key)
	iter := m.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := m.client.Del(ctx, iter.Val()).Err(); err != nil {
			slog.ErrorContext(ctx, "ratelimit: 删除配额键失败", "key", iter.Val(), "error", err)
		}
	}
	if err := iter.Err(); err != nil {
		slog.ErrorContext(ctx, "ratelimit: SCAN 迭代配额键失败", "pattern", pattern, "error", err)
		return err
	}
	return nil
}

func (m *redisQuotaManager) GetUsage(ctx context.Context, quota Quota) (*Usage, error) {
	return m.Check(ctx, quota)
}
