// Package semaphore 提供分布式信号量并发控制.
// 信号量用于限制对共享资源的并发访问数量，
// 适用于分布式部署场景.
// 基本用法:
//
//	counter := semaphore.CacheCounter(cacheClient)
//	sem := semaphore.New(counter, "api-limit", 100)
//	if err := sem.Acquire(ctx); err != nil {
//	    return err
//	}
//	defer sem.Release(ctx)
//
// 中间件:
//
//	endpoint = semaphore.EndpointMiddleware(sem)(endpoint)
package semaphore

import (
	"context"
	"time"
)

// Semaphore 信号量接口.
type Semaphore interface {
	// Acquire 获取一个许可.
	// 如果没有可用许可，会阻塞等待直到获取成功或 context 取消.
	Acquire(ctx context.Context) error

	// TryAcquire 尝试获取一个许可.
	// 如果没有可用许可，立即返回 false.
	TryAcquire(ctx context.Context) bool

	// Release 释放一个许可.
	Release(ctx context.Context) error

	// Available 返回当前可用的许可数量.
	Available(ctx context.Context) (int64, error)

	// Size 返回信号量的总大小.
	Size() int64
}

// Counter 信号量所需的计数器接口.
// 这是 semaphore 包的最小依赖接口.
// 可以用 cache.Cache、Redis 客户端或其他存储实现.
type Counter interface {
	// Increment 原子增加计数并返回新值.
	Increment(ctx context.Context, key string) (int64, error)

	// Decrement 原子减少计数并返回新值.
	Decrement(ctx context.Context, key string) (int64, error)

	// Get 获取当前计数值.
	Get(ctx context.Context, key string) (int64, error)

	// Expire 设置键的过期时间.
	Expire(ctx context.Context, key string, ttl time.Duration) error
}
