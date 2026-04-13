package adaptive_test

import (
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/v2/middleware/adaptive"
)

func ExampleNew() {
	// Create an adaptive limiter with CPU strategy.
	limiter, err := adaptive.New(&adaptive.Config{
		Strategy:     adaptive.StrategyCPU,
		CPUThreshold: 0.8,
		WindowSize:   10 * time.Second,
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// Under normal load, requests are allowed.
	fmt.Println("allowed:", limiter.Allow())
	fmt.Println("strategy:", adaptive.StrategyCPU)
	// Output:
	// allowed: true
	// strategy: cpu
}

func ExampleNew_validation() {
	// Nil config returns an error.
	_, err := adaptive.New(nil)
	fmt.Println(err)
	// Output:
	// adaptive: 配置不能为空
}

func ExampleLimiter_Status() {
	limiter, _ := adaptive.New(&adaptive.Config{
		Strategy:     adaptive.StrategyCPU,
		CPUThreshold: 0.8,
	})

	status := limiter.Status()
	fmt.Println("is limiting:", status.IsLimiting)
	// Output:
	// is limiting: false
}
