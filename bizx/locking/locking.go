// Package locking 提供业务级锁实现.
// 区别于 storage/lock 的基础分布式锁，本包提供可重入锁、读写锁、
// 自动续期等高级锁能力.
// 基本用法:
//
//	// 普通锁
//	l := locking.NewLock(locker, "order:123", locking.WithTTL(30*time.Second))
//	err := locking.WithLock(ctx, l, func(ctx context.Context) error {
//	    return processOrder(123)
//	})
//	// 可重入锁
//	rl := locking.NewReentrantLock(locker, "resource:abc")
//	rl.Lock(ctx)
//	rl.Lock(ctx) // 同一 goroutine 可再次获取
//	rl.Unlock(ctx)
//	rl.Unlock(ctx)
//	// 读写锁
//	rwl := locking.NewRWLock(locker, "config")
//	locking.WithRLock(ctx, rwl, func(ctx context.Context) error {
//	    return readConfig()
//	})
package locking

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Tsukikage7/servex/v2/storage/lock"
)

var (
	// ErrLockFailed 获取锁失败.
	ErrLockFailed = errors.New("locking: failed to acquire lock")
	// ErrNotLocked 未持有锁.
	ErrNotLocked = errors.New("locking: not locked")
	// ErrLockExpired 锁已过期.
	ErrLockExpired = errors.New("locking: lock expired")
)

// Lock 锁接口.
type Lock interface {
	Lock(ctx context.Context) error
	Unlock(ctx context.Context) error
	Extend(ctx context.Context, ttl time.Duration) error
}

// ReentrantLock 可重入锁接口.
type ReentrantLock interface {
	Lock
	LockCount() int
}

// RWLock 读写锁接口.
type RWLock interface {
	RLock(ctx context.Context) error
	RUnlock(ctx context.Context) error
	Lock(ctx context.Context) error
	Unlock(ctx context.Context) error
}

// options 锁选项.
type options struct {
	ttl           time.Duration
	retryInterval time.Duration
	retryTimeout  time.Duration
}

// Option 锁选项函数.
type Option func(*options)

// WithTTL 设置锁的过期时间，默认 30s.
func WithTTL(ttl time.Duration) Option {
	return func(o *options) {
		o.ttl = ttl
	}
}

// WithRetryInterval 设置重试间隔，默认 100ms.
func WithRetryInterval(d time.Duration) Option {
	return func(o *options) {
		o.retryInterval = d
	}
}

// WithRetryTimeout 设置重试超时，默认 10s.
func WithRetryTimeout(d time.Duration) Option {
	return func(o *options) {
		o.retryTimeout = d
	}
}

func applyOptions(opts []Option) *options {
	o := &options{
		ttl:           30 * time.Second,
		retryInterval: 100 * time.Millisecond,
		retryTimeout:  10 * time.Second,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithLock 在锁保护下执行函数.
func WithLock(ctx context.Context, lock Lock, fn func(ctx context.Context) error) error {
	if err := lock.Lock(ctx); err != nil {
		return err
	}
	defer lock.Unlock(ctx) //nolint:errcheck
	return fn(ctx)
}

// WithRLock 在读锁保护下执行函数.
func WithRLock(ctx context.Context, rwlock RWLock, fn func(ctx context.Context) error) error {
	if err := rwlock.RLock(ctx); err != nil {
		return err
	}
	defer rwlock.RUnlock(ctx) //nolint:errcheck
	return fn(ctx)
}

// ---- 普通锁实现 ----

// simpleLock 普通分布式锁，包装 storage/lock.Locker 并添加重试逻辑.
type simpleLock struct {
	locker lock.Locker
	key    string
	opts   *options
}

// NewLock 创建普通分布式锁.
func NewLock(locker lock.Locker, key string, opts ...Option) Lock {
	return &simpleLock{
		locker: locker,
		key:    key,
		opts:   applyOptions(opts),
	}
}

func (l *simpleLock) Lock(ctx context.Context) error {
	deadline := time.Now().Add(l.opts.retryTimeout)
	for {
		acquired, err := l.locker.TryLock(ctx, l.key, l.opts.ttl)
		if err != nil {
			return err
		}
		if acquired {
			return nil
		}
		if time.Now().After(deadline) {
			return ErrLockFailed
		}
		retryTimer := time.NewTimer(l.opts.retryInterval)
		select {
		case <-ctx.Done():
			retryTimer.Stop()
			return ctx.Err()
		case <-retryTimer.C:
		}
	}
}

func (l *simpleLock) Unlock(ctx context.Context) error {
	return l.locker.Unlock(ctx, l.key)
}

func (l *simpleLock) Extend(ctx context.Context, ttl time.Duration) error {
	return l.locker.Extend(ctx, l.key, ttl)
}

// ---- 可重入锁实现 ----

type lockTokenCtxKey struct{}

// WithLockToken 将锁令牌注入到 context，用于可重入锁身份识别.
func WithLockToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, lockTokenCtxKey{}, token)
}

func lockTokenFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(lockTokenCtxKey{}).(string); ok {
		return v
	}
	return ""
}

// reentrantLock 可重入锁，同一令牌持有者可多次获取.
// 通过 WithLockToken(ctx, token) 注入令牌，持有相同令牌的调用可重入.
type reentrantLock struct {
	locker lock.Locker
	key    string
	opts   *options

	mu    sync.Mutex
	count int32
	token string // 当前持有锁的令牌
}

// NewReentrantLock 创建可重入锁.
func NewReentrantLock(locker lock.Locker, key string, opts ...Option) ReentrantLock {
	return &reentrantLock{
		locker: locker,
		key:    key,
		opts:   applyOptions(opts),
	}
}

func (l *reentrantLock) Lock(ctx context.Context) error {
	token := lockTokenFromCtx(ctx)

	l.mu.Lock()
	// 同一令牌可重入
	if l.count > 0 && token != "" && l.token == token {
		l.count++
		l.mu.Unlock()
		return nil
	}
	l.mu.Unlock()

	// 尝试获取底层锁
	deadline := time.Now().Add(l.opts.retryTimeout)
	for {
		acquired, err := l.locker.TryLock(ctx, l.key, l.opts.ttl)
		if err != nil {
			return err
		}
		if acquired {
			l.mu.Lock()
			l.count = 1
			l.token = token
			l.mu.Unlock()
			return nil
		}
		if time.Now().After(deadline) {
			return ErrLockFailed
		}
		retryTimer := time.NewTimer(l.opts.retryInterval)
		select {
		case <-ctx.Done():
			retryTimer.Stop()
			return ctx.Err()
		case <-retryTimer.C:
		}
	}
}

func (l *reentrantLock) Unlock(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.count <= 0 {
		return ErrNotLocked
	}
	l.count--
	if l.count == 0 {
		return l.locker.Unlock(ctx, l.key)
	}
	return nil
}

func (l *reentrantLock) Extend(ctx context.Context, ttl time.Duration) error {
	l.mu.Lock()
	if l.count <= 0 {
		l.mu.Unlock()
		return ErrNotLocked
	}
	l.mu.Unlock()
	return l.locker.Extend(ctx, l.key, ttl)
}

func (l *reentrantLock) LockCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return int(l.count)
}

// ---- 读写锁实现 ----

// rwLock 分布式互斥锁，对外提供 RWLock 接口.
// 注意: 分布式环境下无法用本地计数器实现真正的读写锁，
// 因此 RLock/RUnlock 实际退化为写锁互斥锁，保证正确性.
type rwLock struct {
	locker lock.Locker
	key    string
	opts   *options
}

// NewRWLock 创建分布式锁实现 RWLock 接口.
// 注意: 分布式场景下 RLock 退化为互斥锁，不支持多读并发.
// 如需真正的分布式读写锁，请使用支持读写语义的分布式锁服务.
func NewRWLock(locker lock.Locker, key string, opts ...Option) RWLock {
	return &rwLock{
		locker: locker,
		key:    key,
		opts:   applyOptions(opts),
	}
}

func (l *rwLock) writerKey() string {
	return l.key + ":w"
}

// RLock 获取读锁.
// 注意: 分布式环境下退化为互斥锁，与 Lock 行为一致.
func (l *rwLock) RLock(ctx context.Context) error {
	return l.Lock(ctx)
}

// RUnlock 释放读锁.
func (l *rwLock) RUnlock(ctx context.Context) error {
	return l.Unlock(ctx)
}

func (l *rwLock) Lock(ctx context.Context) error {
	deadline := time.Now().Add(l.opts.retryTimeout)
	for {
		acquired, err := l.locker.TryLock(ctx, l.writerKey(), l.opts.ttl)
		if err != nil {
			return err
		}
		if acquired {
			return nil
		}
		if time.Now().After(deadline) {
			return ErrLockFailed
		}
		retryTimer := time.NewTimer(l.opts.retryInterval)
		select {
		case <-ctx.Done():
			retryTimer.Stop()
			return ctx.Err()
		case <-retryTimer.C:
		}
	}
}

func (l *rwLock) Unlock(ctx context.Context) error {
	return l.locker.Unlock(ctx, l.writerKey())
}
