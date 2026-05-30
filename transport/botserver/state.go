package botserver

import (
	"context"
	"sync"
	"time"
)

// StateStore 对话状态持久化接口。
type StateStore interface {
	Get(chatID string) (string, error)
	Set(chatID string, state string) error
	Del(chatID string) error
}

// redisClient 内部最小化 Redis 接口，避免直接依赖 storage/redis 包。
type redisClient interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value any, expiration time.Duration) error
	Del(ctx context.Context, keys ...string) (int64, error)
}

// memoryStateStore 内存状态存储实现开发/单机场景。
type memoryStateStore struct {
	mu   sync.RWMutex
	data map[string]string
}

// NewMemoryStateStore 创建内存状态存储。
func NewMemoryStateStore() StateStore {
	return &memoryStateStore{data: make(map[string]string)}
}

func (m *memoryStateStore) Get(chatID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data[chatID], nil
}

func (m *memoryStateStore) Set(chatID string, state string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[chatID] = state
	return nil
}

func (m *memoryStateStore) Del(chatID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, chatID)
	return nil
}

// RedisStateOption Redis 状态存储选项。
type RedisStateOption func(*redisStateStore)

// WithKeyPrefix 设置 Redis key 前缀默认 "botstate:"。
func WithKeyPrefix(prefix string) RedisStateOption {
	return func(s *redisStateStore) {
		s.prefix = prefix
	}
}

// redisStateStore Redis 状态存储实现生产多实例场景。
type redisStateStore struct {
	client redisClient
	prefix string
}

// NewRedisStateStore 创建 Redis 状态存储。
func NewRedisStateStore(client redisClient, opts ...RedisStateOption) StateStore {
	s := &redisStateStore{
		client: client,
		prefix: "botstate:",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *redisStateStore) Get(chatID string) (string, error) {
	return s.client.Get(context.Background(), s.prefix+chatID)
}

func (s *redisStateStore) Set(chatID string, state string) error {
	return s.client.Set(context.Background(), s.prefix+chatID, state, 0)
}

func (s *redisStateStore) Del(chatID string) error {
	_, err := s.client.Del(context.Background(), s.prefix+chatID)
	return err
}
