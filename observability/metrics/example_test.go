package metrics_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/observability/metrics"
)

func ExampleDefaultConfig() {
	cfg := metrics.DefaultConfig()
	fmt.Println(cfg.Path)
	fmt.Println(cfg.Namespace)
	// Output:
	// /metrics
	// app
}
