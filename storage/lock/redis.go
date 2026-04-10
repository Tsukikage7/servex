package lock

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Tsukikage7/servex/storage/cache"
)

// Redis 基于 Redis 的分布式锁.
//
// 使用 Redis 的 SETNX + EXPIRE 实现，保证锁的原子性和过期机制.
// 每个锁实例有唯一的 owner ID，确保只有持有者能释放锁.
type Redis struct {
	cache      cache.Cache
	keyPrefix  string
	ownerID    string
	retryWait  time.Duration
	maxRetries int

	// 存储当前持有的锁（key -> true）
	// 用于 Unlock 和 Extend 时验证所有权
	heldMu sync.RWMutex
	held   map[string]bool
}

// RedisOption Redis 锁配置选项.
type RedisOption func(*Redis)

// WithKeyPrefix 设置锁键前缀.
//
// 默认 "lock:".
func WithKeyPrefix(prefix string) RedisOption {
	return func(r *Redis) {
		r.keyPrefix = prefix
	}
}

// WithOwnerID 设置锁持有者 ID.
//
// 默认自动生成 UUID.
// 如果需要在多个实例间共享锁，可以设置相同的 owner ID.
func WithOwnerID(id string) RedisOption {
	return func(r *Redis) {
		r.ownerID = id
	}
}

// WithRetryWait 设置重试等待时间.
//
// Lock 方法获取锁失败时的重试间隔.
// 默认 100ms.
func WithRetryWait(wait time.Duration) RedisOption {
	return func(r *Redis) {
		r.retryWait = wait
	}
}

// WithMaxRetries 设置最大重试次数.
//
// Lock 方法的最大重试次数，0 表示无限重试（直到 context 取消）.
// 默认 0.
func WithMaxRetries(n int) RedisOption {
	return func(r *Redis) {
		r.maxRetries = n
	}
}

// NewRedis 创建 Redis 分布式锁.
func NewRedis(c cache.Cache, opts ...RedisOption) *Redis {
	if c == nil {
		panic("lock: 缓存实例不能为空")
	}

	r := &Redis{
		cache:      c,
		keyPrefix:  "lock:",
		ownerID:    uuid.New().String(),
		retryWait:  100 * time.Millisecond,
		maxRetries: 0,
		held:       make(map[string]bool),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// TryLock 尝试获取锁.
func (r *Redis) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	fullKey := r.keyPrefix + key

	acquired, err := r.cache.TryLock(ctx, fullKey, r.ownerID, ttl)
	if err != nil {
		return false, err
	}

	if acquired {
		r.heldMu.Lock()
		r.held[key] = true
		r.heldMu.Unlock()
	}

	return acquired, nil
}

// Lock 获取锁（阻塞）.
func (r *Redis) Lock(ctx context.Context, key string, ttl time.Duration) error {
	retries := 0

	for {
		acquired, err := r.TryLock(ctx, key, ttl)
		if err != nil {
			return err
		}
		if acquired {
			return nil
		}

		retries++
		if r.maxRetries > 0 && retries >= r.maxRetries {
			return ErrLockNotAcquired
		}

		// 每次循环创建新 timer，避免 Reset 竞态
		timer := time.NewTimer(r.retryWait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			// 重试
		}
	}
}

// Unlock 释放锁.
func (r *Redis) Unlock(ctx context.Context, key string) error {
	// 检查是否持有该锁
	r.heldMu.RLock()
	held := r.held[key]
	r.heldMu.RUnlock()
	if !held {
		return ErrLockNotHeld
	}

	fullKey := r.keyPrefix + key

	err := r.cache.Unlock(ctx, fullKey, r.ownerID)
	if err != nil {
		// 如果是锁不存在或不是持有者，可能已过期
		if err == cache.ErrLockNotHeld {
			r.heldMu.Lock()
			delete(r.held, key)
			r.heldMu.Unlock()
			return ErrLockNotHeld
		}
		return err
	}

	r.heldMu.Lock()
	delete(r.held, key)
	r.heldMu.Unlock()
	return nil
}

// Extend 延长锁的过期时间.
func (r *Redis) Extend(ctx context.Context, key string, ttl time.Duration) error {
	// 检查是否持有该锁
	r.heldMu.RLock()
	held := r.held[key]
	r.heldMu.RUnlock()
	if !held {
		return ErrLockNotHeld
	}

	fullKey := r.keyPrefix + key

	// 原子地验证持有者并延长过期时间
	extended, err := r.cache.ExtendLock(ctx, fullKey, r.ownerID, ttl)
	if err != nil {
		return err
	}
	if !extended {
		r.heldMu.Lock()
		delete(r.held, key)
		r.heldMu.Unlock()
		return ErrLockExpired
	}

	return nil
}

// OwnerID 返回当前锁持有者 ID.
func (r *Redis) OwnerID() string {
	return r.ownerID
}

// IsHeld 检查是否持有指定的锁.
func (r *Redis) IsHeld(key string) bool {
	r.heldMu.RLock()
	defer r.heldMu.RUnlock()
	return r.held[key]
}
