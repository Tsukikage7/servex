package locking_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Tsukikage7/servex/v2/storage/lock"

	"github.com/Tsukikage7/servex/v2/bizx/locking"
)

// testLocker 用于示例的简易内存锁实现.
type testLocker struct {
	mu    sync.Mutex
	locks map[string]bool
}

func newTestLocker() *testLocker {
	return &testLocker{locks: make(map[string]bool)}
}

func (l *testLocker) TryLock(_ context.Context, key string, _ time.Duration) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.locks[key] {
		return false, nil
	}
	l.locks[key] = true
	return true, nil
}

func (l *testLocker) Lock(ctx context.Context, key string, ttl time.Duration) error {
	for {
		ok, err := l.TryLock(ctx, key, ttl)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func (l *testLocker) Unlock(_ context.Context, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.locks, key)
	return nil
}

func (l *testLocker) Extend(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

var _ lock.Locker = (*testLocker)(nil)

func ExampleWithLock() {
	locker := newTestLocker()
	l := locking.NewLock(locker, "order:123", locking.WithTTL(5*time.Second))

	err := locking.WithLock(context.Background(), l, func(ctx context.Context) error {
		fmt.Println("processing order under lock")
		return nil
	})
	fmt.Println("error:", err)
	// Output:
	// processing order under lock
	// error: <nil>
}

func ExampleNewReentrantLock() {
	locker := newTestLocker()
	rl := locking.NewReentrantLock(locker, "resource:abc")
	ctx := locking.WithLockToken(context.Background(), "holder-1")

	_ = rl.Lock(ctx)
	fmt.Println("lock count after first lock:", rl.LockCount())

	_ = rl.Lock(ctx)
	fmt.Println("lock count after second lock:", rl.LockCount())

	_ = rl.Unlock(ctx)
	fmt.Println("lock count after first unlock:", rl.LockCount())

	_ = rl.Unlock(ctx)
	fmt.Println("lock count after second unlock:", rl.LockCount())
	// Output:
	// lock count after first lock: 1
	// lock count after second lock: 2
	// lock count after first unlock: 1
	// lock count after second unlock: 0
}
