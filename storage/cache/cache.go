// Package cache 提供统一的缓存接口和实现.
package cache

import (
	"context"
	"time"
)

// 缓存类型常量.
const (
	// TypeRedis Redis 缓存类型.
	TypeRedis = "redis"
	// TypeMemory 内存缓存类型.
	TypeMemory = "memory"
)

// 默认配置值.
const (
	// DefaultPoolSize 默认连接池大小.
	DefaultPoolSize = 10
	// DefaultTimeout 默认超时时间.
	DefaultTimeout = 5 * time.Second
	// DefaultReadTimeout 默认读取超时时间.
	DefaultReadTimeout = 3 * time.Second
	// DefaultWriteTimeout 默认写入超时时间.
	DefaultWriteTimeout = 3 * time.Second
	// DefaultMaxRetries 默认最大重试次数.
	DefaultMaxRetries = 3
)

// Cache 缓存接口.
type Cache interface {
	// Set 设置键值对.
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	// Get 获取值.
	Get(ctx context.Context, key string) (string, error)
	// Del 删除键.
	Del(ctx context.Context, keys ...string) error
	// Exists 检查键是否存在.
	Exists(ctx context.Context, key string) (bool, error)

	// SetNX 仅当键不存在时设置.
	SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)
	// Increment 递增.
	Increment(ctx context.Context, key string) (int64, error)
	// IncrementBy 增加指定值.
	IncrementBy(ctx context.Context, key string, value int64) (int64, error)
	// Decrement 递减.
	Decrement(ctx context.Context, key string) (int64, error)

	// Expire 设置过期时间.
	Expire(ctx context.Context, key string, ttl time.Duration) error
	// TTL 获取剩余过期时间.
	TTL(ctx context.Context, key string) (time.Duration, error)

	// TryLock 尝试获取分布式锁.
	TryLock(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	// Unlock 释放分布式锁.
	Unlock(ctx context.Context, key string, value string) error
	// ExtendLock 原子地验证锁持有者并延长过期时间.
	//
	// 只有当 key 的值等于 value 时才延长过期时间，避免 GET+EXPIRE 的 TOCTOU 竞态.
	ExtendLock(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)

	// MGet 批量获取.
	MGet(ctx context.Context, keys ...string) ([]string, error)
	// MSet 批量设置.
	MSet(ctx context.Context, pairs map[string]any, ttl time.Duration) error

	// Ping 测试连接.
	Ping(ctx context.Context) error
	// Close 关闭连接.
	Close() error
	// Client 返回底层客户端.
	Client() any
}
