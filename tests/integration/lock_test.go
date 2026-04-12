//go:build integration

package integration

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tsukikage7/servex/storage/cache"
	"github.com/Tsukikage7/servex/storage/lock"
	"github.com/Tsukikage7/servex/testx"
)

func newLockCache(t *testing.T) cache.Cache {
	t.Helper()

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	cfg := cache.NewRedisConfig(addr)
	c, err := cache.NewRedisCache(cfg, testx.NopLogger())
	if err != nil {
		t.Skipf("Redis not available for lock test: %v", err)
		return nil
	}

	t.Cleanup(func() { c.Close() })
	return c
}

func TestLock_Integration(t *testing.T) {
	c := newLockCache(t)
	ctx := t.Context()

	t.Run("TryLock_Unlock", func(t *testing.T) {
		locker := lock.NewRedis(c, lock.WithKeyPrefix("inttest:lock:"))

		acquired, err := locker.TryLock(ctx, "res1", 10*time.Second)
		require.NoError(t, err)
		assert.True(t, acquired)
		assert.True(t, locker.IsHeld("res1"))

		// Same locker trying again should fail (already held in Redis)
		locker2 := lock.NewRedis(c, lock.WithKeyPrefix("inttest:lock:"))
		acquired2, err := locker2.TryLock(ctx, "res1", 10*time.Second)
		require.NoError(t, err)
		assert.False(t, acquired2)

		// Unlock
		err = locker.Unlock(ctx, "res1")
		require.NoError(t, err)
		assert.False(t, locker.IsHeld("res1"))

		// Now locker2 can acquire
		acquired2, err = locker2.TryLock(ctx, "res1", 10*time.Second)
		require.NoError(t, err)
		assert.True(t, acquired2)

		err = locker2.Unlock(ctx, "res1")
		require.NoError(t, err)
	})

	t.Run("Lock_Blocking", func(t *testing.T) {
		locker1 := lock.NewRedis(c, lock.WithKeyPrefix("inttest:lock:blocking:"))
		locker2 := lock.NewRedis(c, lock.WithKeyPrefix("inttest:lock:blocking:"), lock.WithRetryWait(50*time.Millisecond))

		// locker1 acquires
		err := locker1.Lock(ctx, "res", 2*time.Second)
		require.NoError(t, err)

		// locker2 blocks, then acquires after locker1 releases
		done := make(chan error, 1)
		go func() {
			lockCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			done <- locker2.Lock(lockCtx, "res", 5*time.Second)
		}()

		// Release after a short delay
		time.Sleep(200 * time.Millisecond)
		err = locker1.Unlock(ctx, "res")
		require.NoError(t, err)

		// locker2 should succeed
		err = <-done
		require.NoError(t, err)
		assert.True(t, locker2.IsHeld("res"))

		err = locker2.Unlock(ctx, "res")
		require.NoError(t, err)
	})

	t.Run("Lock_ContextCancel", func(t *testing.T) {
		locker1 := lock.NewRedis(c, lock.WithKeyPrefix("inttest:lock:cancel:"))
		locker2 := lock.NewRedis(c, lock.WithKeyPrefix("inttest:lock:cancel:"), lock.WithRetryWait(50*time.Millisecond))

		err := locker1.Lock(ctx, "res", 30*time.Second)
		require.NoError(t, err)
		defer locker1.Unlock(ctx, "res")

		// locker2 tries with a short timeout
		lockCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
		defer cancel()

		err = locker2.Lock(lockCtx, "res", 5*time.Second)
		assert.Error(t, err) // should be context deadline exceeded
	})

	t.Run("Extend", func(t *testing.T) {
		locker := lock.NewRedis(c, lock.WithKeyPrefix("inttest:lock:extend:"))

		acquired, err := locker.TryLock(ctx, "res", 2*time.Second)
		require.NoError(t, err)
		assert.True(t, acquired)

		// Extend the lock
		err = locker.Extend(ctx, "res", 10*time.Second)
		require.NoError(t, err)

		// Verify lock is still held
		assert.True(t, locker.IsHeld("res"))

		err = locker.Unlock(ctx, "res")
		require.NoError(t, err)
	})

	t.Run("Unlock_NotHeld", func(t *testing.T) {
		locker := lock.NewRedis(c, lock.WithKeyPrefix("inttest:lock:notheld:"))

		err := locker.Unlock(ctx, "nonexistent")
		assert.ErrorIs(t, err, lock.ErrLockNotHeld)
	})

	t.Run("Concurrent", func(t *testing.T) {
		const workers = 10
		var counter atomic.Int64
		var wg sync.WaitGroup

		for i := 0; i < workers; i++ {
			wg.Go(func() {
				locker := lock.NewRedis(c,
					lock.WithKeyPrefix("inttest:lock:concurrent:"),
					lock.WithRetryWait(20*time.Millisecond),
				)
				lockCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()

				err := locker.Lock(lockCtx, "shared", 2*time.Second)
				if err != nil {
					return
				}
				defer locker.Unlock(lockCtx, "shared")

				// Simulate critical section
				val := counter.Load()
				time.Sleep(5 * time.Millisecond)
				counter.Store(val + 1)
			})
		}

		wg.Wait()
		assert.Equal(t, int64(workers), counter.Load())
	})

	t.Run("WithLock", func(t *testing.T) {
		locker := lock.NewRedis(c, lock.WithKeyPrefix("inttest:lock:withlock:"))

		var executed bool
		err := lock.WithLock(ctx, locker, "task", 5*time.Second, func() error {
			executed = true
			return nil
		})
		require.NoError(t, err)
		assert.True(t, executed)
	})

	t.Run("TryWithLock", func(t *testing.T) {
		locker1 := lock.NewRedis(c, lock.WithKeyPrefix("inttest:lock:trywith:"))
		locker2 := lock.NewRedis(c, lock.WithKeyPrefix("inttest:lock:trywith:"))

		// locker1 holds the lock
		acquired, err := locker1.TryLock(ctx, "task", 10*time.Second)
		require.NoError(t, err)
		assert.True(t, acquired)
		defer locker1.Unlock(ctx, "task")

		// locker2 should fail immediately
		err = lock.TryWithLock(ctx, locker2, "task", 5*time.Second, func() error {
			t.Fatal("should not execute")
			return nil
		})
		assert.ErrorIs(t, err, lock.ErrLockNotAcquired)
	})
}
