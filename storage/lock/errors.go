package lock

import "errors"

var (
	// ErrLockNotAcquired 无法获取锁.
	ErrLockNotAcquired = errors.New("lock: failed to acquire lock")

	// ErrLockNotHeld 锁未被持有释放或延长时.
	ErrLockNotHeld = errors.New("lock: lock not held")

	// ErrLockExpired 锁已过期.
	ErrLockExpired = errors.New("lock: lock expired")
)
