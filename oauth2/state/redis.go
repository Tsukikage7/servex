package state

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/Tsukikage7/servex/v2/storage/cache"
)

// RedisStore 基于 Redis 的 StateStore，用于生产环境.
// 通过 servex 的 cache.Cache 接口操作，支持 Redis 和内存缓存.
type RedisStore struct {
	cache  cache.Cache
	prefix string
	ttl    time.Duration
}

// RedisOption 配置 RedisStore 的选项函数.
type RedisOption func(*RedisStore)

// WithPrefix 设置缓存键前缀.
func WithPrefix(prefix string) RedisOption {
	return func(s *RedisStore) { s.prefix = prefix }
}

// WithTTL 设置 state 的过期时间.
func WithTTL(ttl time.Duration) RedisOption {
	return func(s *RedisStore) { s.ttl = ttl }
}

// NewRedisStore 创建基于缓存的 StateStore.
// 接受 servex 的 cache.Cache（Redis 或内存均可），复用已有连接.
func NewRedisStore(c cache.Cache, opts ...RedisOption) (*RedisStore, error) {
	if c == nil {
		return nil, errors.New("oauth2/state: cache 不能为空")
	}
	s := &RedisStore{
		cache:  c,
		prefix: "oauth2:state:",
		ttl:    10 * time.Minute,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// Generate 生成一个新的 state 参数并存入缓存.
func (s *RedisStore) Generate(ctx context.Context) (string, error) {
	state := uuid.NewString()
	if err := s.cache.Set(ctx, s.prefix+state, "1", s.ttl); err != nil {
		return "", err
	}
	return state, nil
}

// Validate 验证并消费一个 state 参数.
// 使用 Get+Del 实现一次性消费；通过 SetNX 标记防止并发重复消费.
func (s *RedisStore) Validate(ctx context.Context, state string) (bool, error) {
	key := s.prefix + state
	// 使用 SetNX 设置消费标记，确保只有一个请求能成功消费该 state
	consumeKey := key + ":consumed"
	acquired, err := s.cache.SetNX(ctx, consumeKey, "1", s.ttl)
	if err != nil {
		return false, err
	}
	if !acquired {
		// 已被其他请求消费
		return false, nil
	}

	val, err := s.cache.Get(ctx, key)
	if err != nil {
		if errors.Is(err, cache.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	// 删除原始 state key
	_ = s.cache.Del(ctx, key)
	return val == "1", nil
}
