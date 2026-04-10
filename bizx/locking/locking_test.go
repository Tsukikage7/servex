package locking

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLocker 模拟的分布式锁实现.
type mockLocker struct {
	mu    sync.Mutex
	locks map[string]bool
}

func newMockLocker() *mockLocker {
	return &mockLocker{locks: make(map[string]bool)}
}

func (m *mockLocker) TryLock(_ context.Context, key string, _ time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.locks[key] {
		return false, nil
	}
	m.locks[key] = true
	return true, nil
}

func (m *mockLocker) Lock(ctx context.Context, key string, ttl time.Duration) error {
	for {
		acquired, err := m.TryLock(ctx, key, ttl)
		if err != nil {
			return err
		}
		if acquired {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func (m *mockLocker) Unlock(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.locks, key)
	return nil
}

func (m *mockLocker) Extend(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func TestLock_Unlock(t *testing.T) {
	ctx := context.Background()
	locker := newMockLocker()
	l := NewLock(locker, "test:key", WithTTL(5*time.Second), WithRetryTimeout(1*time.Second))

	err := l.Lock(ctx)
	require.NoError(t, err)

	err = l.Unlock(ctx)
	require.NoError(t, err)

	// 再次加锁应该成功
	err = l.Lock(ctx)
	require.NoError(t, err)
	_ = l.Unlock(ctx)
}

func TestWithLock(t *testing.T) {
	ctx := context.Background()
	locker := newMockLocker()
	l := NewLock(locker, "test:with", WithTTL(5*time.Second))

	executed := false
	err := WithLock(ctx, l, func(ctx context.Context) error {
		executed = true
		return nil
	})
	require.NoError(t, err)
	assert.True(t, executed)
}

func TestReentrantLock(t *testing.T) {
	ctx := context.Background()
	locker := newMockLocker()
	rl := NewReentrantLock(locker, "test:reentrant", WithTTL(5*time.Second))

	// 使用 token 标识持有者
	ctx = WithLockToken(ctx, "holder-1")

	// 第一次加锁
	err := rl.Lock(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, rl.LockCount())

	// 第二次加锁（相同 token 可重入）
	err = rl.Lock(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, rl.LockCount())

	// 第一次解锁
	err = rl.Unlock(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, rl.LockCount())

	// 第二次解锁
	err = rl.Unlock(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, rl.LockCount())

	// 未持有锁时解锁
	err = rl.Unlock(ctx)
	assert.ErrorIs(t, err, ErrNotLocked)
}

func TestRWLock_ReadConcurrent(t *testing.T) {
	ctx := context.Background()
	locker := newMockLocker()
	rwl := NewRWLock(locker, "test:rw", WithTTL(5*time.Second))

	// 多个读者可以同时获取读锁
	var wg sync.WaitGroup
	readCount := 0
	mu := sync.Mutex{}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := rwl.RLock(ctx)
			require.NoError(t, err)
			mu.Lock()
			readCount++
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			err = rwl.RUnlock(ctx)
			require.NoError(t, err)
		}()
	}

	wg.Wait()
	assert.Equal(t, 5, readCount)
}
