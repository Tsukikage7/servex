// Package lock 提供分布式锁实现.
//
// 分布式锁用于在多个进程/服务之间协调对共享资源的访问，
// 确保同一时间只有一个客户端能够执行特定操作.
//
// 基本用法:
//
//	locker := lock.NewRedis(cacheClient)
//
//	// 方式1: 使用 WithLock 辅助函数
//	err := lock.WithLock(ctx, locker, "my-resource", 30*time.Second, func() error {
//	    // 执行需要互斥的操作
//	    return doSomething()
//	})
//
//	// 方式2: 手动管理锁
//	acquired, err := locker.TryLock(ctx, "my-resource", 30*time.Second)
//	if err != nil {
//	    return err
//	}
//	if !acquired {
//	    return errors.New("failed to acquire lock")
//	}
//	defer locker.Unlock(ctx, "my-resource")
//
//	// 执行操作...
//
// 阻塞获取锁:
//
//	// Lock 会阻塞直到获取成功或超时
//	err := locker.Lock(ctx, "my-resource", 30*time.Second)
//	if err != nil {
//	    return err
//	}
//	defer locker.Unlock(ctx, "my-resource")
package lock

import (
	"context"
	"errors"
	"time"
)

// Locker 分布式锁接口.
type Locker interface {
	// TryLock 尝试获取锁.
	//
	// 如果锁已被持有，立即返回 false.
	// key: 锁的唯一标识
	// ttl: 锁的过期时间防止死锁
	TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error)

	// Lock 获取锁阻塞.
	//
	// 会阻塞等待直到获取成功或 context 取消.
	// key: 锁的唯一标识
	// ttl: 锁的过期时间
	Lock(ctx context.Context, key string, ttl time.Duration) error

	// Unlock 释放锁.
	//
	// 只有锁的持有者才能释放锁.
	// 如果锁不存在或不是当前持有者，返回 ErrLockNotHeld.
	Unlock(ctx context.Context, key string) error

	// Extend 延长锁的过期时间.
	//
	// 用于长时间操作，在锁即将过期前延长时间.
	// 只有锁的持有者才能延长.
	Extend(ctx context.Context, key string, ttl time.Duration) error
}

// WithLock 执行带锁保护的操作.
//
// 自动获取锁、执行操作、释放锁.
// 如果获取锁失败，返回 ErrLockNotAcquired.
//
// 示例:
//
//	err := lock.WithLock(ctx, locker, "order:123", 30*time.Second, func() error {
//	    return processOrder(123)
//	})
func WithLock(ctx context.Context, locker Locker, key string, ttl time.Duration, fn func() error) error {
	if err := locker.Lock(ctx, key, ttl); err != nil {
		return err
	}

	fnErr := fn()
	unlockErr := locker.Unlock(ctx, key)

	return errors.Join(fnErr, unlockErr)
}

// TryWithLock 尝试执行带锁保护的操作.
//
// 非阻塞版本，如果无法立即获取锁则返回 ErrLockNotAcquired.
//
// 示例:
//
//	err := lock.TryWithLock(ctx, locker, "order:123", 30*time.Second, func() error {
//	    return processOrder(123)
//	})
//	if errors.Is(err, lock.ErrLockNotAcquired) {
//	    // 锁被占用，稍后重试
//	}
func TryWithLock(ctx context.Context, locker Locker, key string, ttl time.Duration, fn func() error) error {
	acquired, err := locker.TryLock(ctx, key, ttl)
	if err != nil {
		return err
	}
	if !acquired {
		return ErrLockNotAcquired
	}

	fnErr := fn()
	unlockErr := locker.Unlock(ctx, key)

	return errors.Join(fnErr, unlockErr)
}
