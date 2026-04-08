package profiling_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/observability/profiling"
)

func ExampleConfig() {
	cfg := &profiling.Config{
		Enabled: true,
		Types:   []profiling.ProfileType{profiling.ProfileCPU, profiling.ProfileHeap},
	}
	fmt.Println(cfg.Enabled)
	fmt.Println(cfg.Types[0])
	fmt.Println(cfg.Types[1])
	// Output:
	// true
	// cpu
	// heap
}
