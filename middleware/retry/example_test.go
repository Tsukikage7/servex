package retry_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/v2/middleware/retry"
)

func ExampleDo() {
	attempts := 0

	// Retry an operation that succeeds on the 2nd attempt.
	err := retry.Do(context.Background(), func() error {
		attempts++
		if attempts < 2 {
			return errors.New("transient error")
		}
		return nil
	}).WithMaxAttempts(3).WithDelay(time.Millisecond).Run()

	fmt.Println("error:", err)
	fmt.Println("attempts:", attempts)
	// Output:
	// error: <nil>
	// attempts: 2
}

func ExampleDo_maxAttempts() {
	// All attempts fail.
	err := retry.Do(context.Background(), func() error {
		return errors.New("always fails")
	}).WithMaxAttempts(2).WithDelay(time.Millisecond).Run()

	fmt.Println("error:", err)
	// Output:
	// error: 已达到最大重试次数
}

func ExampleExponentialBackoff() {
	base := 100 * time.Millisecond

	fmt.Println("attempt 0:", retry.ExponentialBackoff(0, base))
	fmt.Println("attempt 1:", retry.ExponentialBackoff(1, base))
	fmt.Println("attempt 2:", retry.ExponentialBackoff(2, base))
	// Output:
	// attempt 0: 100ms
	// attempt 1: 200ms
	// attempt 2: 400ms
}
