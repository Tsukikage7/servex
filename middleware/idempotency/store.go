package idempotency

import (
	"context"
	"time"

	"github.com/Tsukikage7/servex/storage/cache"
)

// IdempotentStore 幂等性存储实现.
// 基于 KV 接口实现，适用于分布式部署场景.
type IdempotentStore struct {
	kv        KV
	keyPrefix string
}

// StoreOption 存储配置选项.
type StoreOption func(*IdempotentStore)

// WithKeyPrefix 设置键前缀.
func WithKeyPrefix(prefix string) StoreOption {
	return func(s *IdempotentStore) {
		s.keyPrefix = prefix
	}
}

// NewStore 创建幂等性存储.
// kv: KV 存储实现（可用 CacheKV 适配 cache.Cache）
func NewStore(kv KV, opts ...StoreOption) *IdempotentStore {
	s := &IdempotentStore{
		kv:        kv,
		keyPrefix: "idempotency:",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Get 获取幂等键对应的结果.
func (s *IdempotentStore) Get(ctx context.Context, key string) (*Result, error) {
	fullKey := s.keyPrefix + key

	data, err := s.kv.Get(ctx, fullKey)
	if err != nil {
		// 键不存在不是错误
		return nil, nil
	}
	if data == "" {
		return nil, nil
	}

	return DecodeResult([]byte(data))
}

// Set 设置幂等键和结果.
func (s *IdempotentStore) Set(ctx context.Context, key string, result *Result, ttl time.Duration) error {
	fullKey := s.keyPrefix + key
	lockKey := s.keyPrefix + "lock:" + key

	data, err := result.Encode()
	if err != nil {
		return err
	}

	// 设置结果
	if err := s.kv.Set(ctx, fullKey, string(data), ttl); err != nil {
		return err
	}

	// 删除锁
	_ = s.kv.Del(ctx, lockKey)

	return nil
}

// SetNX 仅在键不存在时设置（用于获取处理锁）.
// 直接依赖 SetNX 的原子性，避免 Exists + SetNX 之间的 TOCTOU 竞态窗口.
func (s *IdempotentStore) SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	fullKey := s.keyPrefix + key
	lockKey := s.keyPrefix + "lock:" + key

	// 先检查是否已有结果（只读检查，有结果直接返回，无需加锁）
	data, err := s.kv.Get(ctx, fullKey)
	if err == nil && data != "" {
		return false, nil
	}

	// 尝试原子获取锁
	return s.kv.SetNX(ctx, lockKey, "1", ttl)
}

// Delete 删除幂等键.
func (s *IdempotentStore) Delete(ctx context.Context, key string) error {
	fullKey := s.keyPrefix + key
	lockKey := s.keyPrefix + "lock:" + key
	return s.kv.Del(ctx, fullKey, lockKey)
}

// cacheKV 是 cache.Cache 到 KV 的适配器.
type cacheKV struct {
	cache cache.Cache
}

// CacheKV 将 cache.Cache 适配为 KV 接口.
// 示例:
//	redisCache, _ := cache.New(&cache.Config{Type: "redis", ...})
//	kv := idempotency.CacheKV(redisCache)
//	store := idempotency.NewStore(kv)
func CacheKV(c cache.Cache) KV {
	return &cacheKV{cache: c}
}

func (c *cacheKV) Get(ctx context.Context, key string) (string, error) {
	return c.cache.Get(ctx, key)
}

func (c *cacheKV) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.cache.Set(ctx, key, value, ttl)
}

func (c *cacheKV) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return c.cache.SetNX(ctx, key, value, ttl)
}

func (c *cacheKV) Exists(ctx context.Context, key string) (bool, error) {
	return c.cache.Exists(ctx, key)
}

func (c *cacheKV) Del(ctx context.Context, keys ...string) error {
	return c.cache.Del(ctx, keys...)
}
