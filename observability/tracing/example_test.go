package tracing_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/observability/tracing"
)

func ExampleTracingConfig() {
	cfg := &tracing.TracingConfig{
		Enabled:      true,
		SamplingRate: 0.5,
		OTLP: &tracing.OTLPConfig{
			Endpoint: "localhost:4317",
		},
	}
	fmt.Println(cfg.Enabled)
	fmt.Println(cfg.SamplingRate)
	fmt.Println(cfg.OTLP.Endpoint)
	// Output:
	// true
	// 0.5
	// localhost:4317
}
