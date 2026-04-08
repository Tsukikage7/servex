package slo_test

import (
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/observability/slo"
)

func ExampleObjective() {
	obj := &slo.Objective{
		Name:        "api-availability",
		Target:      0.999,
		Window:      30 * 24 * time.Hour,
		Description: "API availability must be 99.9% over 30 days",
	}
	fmt.Println(obj.Name)
	fmt.Println(obj.Target)
	// Output:
	// api-availability
	// 0.999
}

func ExampleNewTracker() {
	obj := &slo.Objective{
		Name:   "latency-p99",
		Target: 0.99,
		Window: 24 * time.Hour,
	}
	tracker, err := slo.NewTracker([]*slo.Objective{obj})
	fmt.Println(err)
	fmt.Println(tracker != nil)
	// Output:
	// <nil>
	// true
}
