package lock

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/storage/cache"
	"github.com/Tsukikage7/servex/v2/testx"
)

// newTestLocker 创建测试用的锁.
func newTestLocker(opts ...RedisOption) (*Redis, cache.Cache) {
	memCache, _ := cache.NewMemoryCache(nil, testx.NopLogger())
	return NewRedis(memCache, opts...), memCache
}

func TestTryLock(t *testing.T) {
	locker, memCache := newTestLocker()
	defer memCache.Close()

	ctx := t.Context()

	t.Run("acquire lock", func(t *testing.T) {
		acquired, err := locker.TryLock(ctx, "test-key", time.Minute)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !acquired {
			t.Error("expected to acquire lock")
		}

		// 验证锁已持有
		if !locker.IsHeld("test-key") {
			t.Error("expected lock to be held")
		}

		// 释放
		_ = locker.Unlock(ctx, "test-key")
	})

	t.Run("lock already held", func(t *testing.T) {
		// 获取锁
		acquired, _ := locker.TryLock(ctx, "held-key", time.Minute)
		if !acquired {
			t.Fatal("failed to acquire lock")
		}

		// 创建另一个 locker 尝试获取同一个锁
		locker2, _ := newTestLocker()
		locker2.cache = locker.cache // 共享缓存

		acquired2, err := locker2.TryLock(ctx, "held-key", time.Minute)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if acquired2 {
			t.Error("expected lock to fail (already held)")
		}

		// 释放
		_ = locker.Unlock(ctx, "held-key")
	})
}

func TestLock(t *testing.T) {
	locker, memCache := newTestLocker(WithRetryWait(10 * time.Millisecond))
	defer memCache.Close()

	ctx := t.Context()

	t.Run("blocking acquire", func(t *testing.T) {
		err := locker.Lock(ctx, "blocking-key", time.Minute)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// 释放
		_ = locker.Unlock(ctx, "blocking-key")
	})

	t.Run("context cancellation", func(t *testing.T) {
		// 先获取锁
		_, _ = locker.TryLock(ctx, "cancel-key", time.Minute)

		// 创建另一个 locker
		locker2, _ := newTestLocker(WithRetryWait(10 * time.Millisecond))
		locker2.cache = locker.cache

		cancelCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		err := locker2.Lock(cancelCtx, "cancel-key", time.Minute)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected DeadlineExceeded, got %v", err)
		}

		// 释放
		_ = locker.Unlock(ctx, "cancel-key")
	})

	t.Run("max retries", func(t *testing.T) {
		// 先获取锁
		_, _ = locker.TryLock(ctx, "retry-key", time.Minute)

		// 创建有最大重试次数的 locker
		locker2, _ := newTestLocker(
			WithRetryWait(10*time.Millisecond),
			WithMaxRetries(3),
		)
		locker2.cache = locker.cache

		err := locker2.Lock(ctx, "retry-key", time.Minute)
		if !errors.Is(err, ErrLockNotAcquired) {
			t.Errorf("expected ErrLockNotAcquired, got %v", err)
		}

		// 释放
		_ = locker.Unlock(ctx, "retry-key")
	})
}

func TestUnlock(t *testing.T) {
	locker, memCache := newTestLocker()
	defer memCache.Close()

	ctx := t.Context()

	t.Run("successful unlock", func(t *testing.T) {
		_, _ = locker.TryLock(ctx, "unlock-key", time.Minute)

		err := locker.Unlock(ctx, "unlock-key")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// 验证锁已释放
		if locker.IsHeld("unlock-key") {
			t.Error("expected lock to be released")
		}
	})

	t.Run("unlock not held", func(t *testing.T) {
		err := locker.Unlock(ctx, "not-held-key")
		if !errors.Is(err, ErrLockNotHeld) {
			t.Errorf("expected ErrLockNotHeld, got %v", err)
		}
	})
}

func TestExtend(t *testing.T) {
	locker, memCache := newTestLocker()
	defer memCache.Close()

	ctx := t.Context()

	t.Run("successful extend", func(t *testing.T) {
		_, _ = locker.TryLock(ctx, "extend-key", time.Minute)

		err := locker.Extend(ctx, "extend-key", 2*time.Minute)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// 释放
		_ = locker.Unlock(ctx, "extend-key")
	})

	t.Run("extend not held", func(t *testing.T) {
		err := locker.Extend(ctx, "not-held-key", time.Minute)
		if !errors.Is(err, ErrLockNotHeld) {
			t.Errorf("expected ErrLockNotHeld, got %v", err)
		}
	})
}

func TestWithLock(t *testing.T) {
	locker, memCache := newTestLocker()
	defer memCache.Close()

	ctx := t.Context()

	t.Run("successful execution", func(t *testing.T) {
		executed := false

		err := WithLock(ctx, locker, "with-lock-key", time.Minute, func() error {
			executed = true
			return nil
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !executed {
			t.Error("expected function to be executed")
		}

		// 验证锁已释放
		if locker.IsHeld("with-lock-key") {
			t.Error("expected lock to be released")
		}
	})

	t.Run("function error", func(t *testing.T) {
		expectedErr := errors.New("test error")

		err := WithLock(ctx, locker, "error-key", time.Minute, func() error {
			return expectedErr
		})

		if !errors.Is(err, expectedErr) {
			t.Errorf("expected test error, got %v", err)
		}

		// 验证锁已释放（即使函数返回错误）
		if locker.IsHeld("error-key") {
			t.Error("expected lock to be released after error")
		}
	})
}

func TestTryWithLock(t *testing.T) {
	locker, memCache := newTestLocker()
	defer memCache.Close()

	ctx := t.Context()

	t.Run("successful execution", func(t *testing.T) {
		executed := false

		err := TryWithLock(ctx, locker, "try-with-key", time.Minute, func() error {
			executed = true
			return nil
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !executed {
			t.Error("expected function to be executed")
		}
	})

	t.Run("lock not available", func(t *testing.T) {
		// 先获取锁
		_, _ = locker.TryLock(ctx, "occupied-key", time.Minute)

		// 创建另一个 locker
		locker2, _ := newTestLocker()
		locker2.cache = locker.cache

		executed := false
		err := TryWithLock(ctx, locker2, "occupied-key", time.Minute, func() error {
			executed = true
			return nil
		})

		if !errors.Is(err, ErrLockNotAcquired) {
			t.Errorf("expected ErrLockNotAcquired, got %v", err)
		}
		if executed {
			t.Error("expected function to not be executed")
		}

		// 释放
		_ = locker.Unlock(ctx, "occupied-key")
	})
}

func TestConcurrency(t *testing.T) {
	locker, memCache := newTestLocker(WithRetryWait(5 * time.Millisecond))
	defer memCache.Close()

	ctx := t.Context()
	var counter atomic.Int32
	var maxConcurrent atomic.Int32
	var current atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Go(func() {
			err := WithLock(ctx, locker, "concurrent-key", time.Minute, func() error {
				c := current.Add(1)
				for {
					max := maxConcurrent.Load()
					if c <= max || maxConcurrent.CompareAndSwap(max, c) {
						break
					}
				}

				counter.Add(1)
				time.Sleep(5 * time.Millisecond)

				current.Add(-1)
				return nil
			})

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}

	wg.Wait()

	// 验证所有操作都执行了
	if counter.Load() != 50 {
		t.Errorf("expected 50 executions, got %d", counter.Load())
	}

	// 验证同一时间只有一个在执行（互斥）
	if maxConcurrent.Load() > 1 {
		t.Errorf("expected max concurrent 1, got %d", maxConcurrent.Load())
	}
}

func TestOptions(t *testing.T) {
	t.Run("custom owner ID", func(t *testing.T) {
		locker, memCache := newTestLocker(WithOwnerID("custom-owner"))
		defer memCache.Close()

		if locker.OwnerID() != "custom-owner" {
			t.Errorf("expected owner ID 'custom-owner', got %s", locker.OwnerID())
		}
	})

	t.Run("custom key prefix", func(t *testing.T) {
		locker, memCache := newTestLocker(WithKeyPrefix("myapp:lock:"))
		defer memCache.Close()

		ctx := t.Context()
		_, _ = locker.TryLock(ctx, "test", time.Minute)

		// 验证键前缀
		exists, _ := locker.cache.Exists(ctx, "myapp:lock:test")
		if !exists {
			t.Error("expected key with custom prefix to exist")
		}

		_ = locker.Unlock(ctx, "test")
	})
}

func TestPanicOnNilCache(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		}
	}()

	NewRedis(nil)
}
