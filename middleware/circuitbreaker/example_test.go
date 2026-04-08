package circuitbreaker_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/Tsukikage7/servex/middleware/circuitbreaker"
)

func ExampleNew() {
	// Create a circuit breaker with default settings.
	cb := circuitbreaker.New(
		circuitbreaker.WithFailureThreshold(3),
		circuitbreaker.WithSuccessThreshold(1),
	)

	fmt.Println("state:", cb.State())

	// Execute a successful operation.
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	fmt.Println("error:", err)
	fmt.Println("state:", cb.State())
	// Output:
	// state: closed
	// error: <nil>
	// state: closed
}

func ExampleBreaker_Execute_failure() {
	cb := circuitbreaker.New(
		circuitbreaker.WithFailureThreshold(2),
	)

	fail := errors.New("service down")

	// Two consecutive failures trip the breaker.
	_ = cb.Execute(context.Background(), func() error { return fail })
	_ = cb.Execute(context.Background(), func() error { return fail })

	fmt.Println("state:", cb.State())
	// Output:
	// state: open
}

func ExampleBreaker_Reset() {
	cb := circuitbreaker.New(circuitbreaker.WithFailureThreshold(1))

	// Trip the breaker.
	_ = cb.Execute(context.Background(), func() error { return errors.New("fail") })
	fmt.Println("before reset:", cb.State())

	// Manually reset.
	cb.Reset()
	fmt.Println("after reset:", cb.State())
	// Output:
	// before reset: open
	// after reset: closed
}
