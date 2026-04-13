package health_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/transport/health"
)

func ExampleNew() {
	// Create a health checker with no registered checks.
	h := health.New()

	// Without checkers, liveness defaults to UP.
	resp := h.Liveness(context.Background())
	fmt.Println("status:", resp.Status)
	// Output:
	// status: UP
}

func ExampleHealth_Readiness() {
	// Create health checker with a readiness checker.
	h := health.New(
		health.WithReadinessChecker(
			health.NewCheckerFunc("test-dep", func(ctx context.Context) health.CheckResult {
				return health.CheckResult{Status: health.StatusUp}
			}),
		),
	)

	resp := h.Readiness(context.Background())
	fmt.Println("status:", resp.Status)
	fmt.Println("checks:", len(resp.Checks))
	// Output:
	// status: UP
	// checks: 1
}

func ExampleNewCheckerFunc() {
	checker := health.NewCheckerFunc("custom", func(ctx context.Context) health.CheckResult {
		return health.CheckResult{
			Status:  health.StatusUp,
			Message: "all good",
		}
	})

	fmt.Println("name:", checker.Name())
	result := checker.Check(context.Background())
	fmt.Println("status:", result.Status)
	fmt.Println("message:", result.Message)
	// Output:
	// name: custom
	// status: UP
	// message: all good
}

func ExampleHealth_IsHealthy() {
	h := health.New()
	fmt.Println("healthy:", h.IsHealthy(context.Background()))
	// Output:
	// healthy: true
}
