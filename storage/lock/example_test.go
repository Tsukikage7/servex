package lock_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/storage/lock"
)

func ExampleErrLockNotAcquired() {
	// Show error messages for lock package errors.
	fmt.Println(lock.ErrLockNotAcquired)
	fmt.Println(lock.ErrLockNotHeld)
	// Output:
	// lock: failed to acquire lock
	// lock: lock not held
}
