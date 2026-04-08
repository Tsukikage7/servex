package logging_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/middleware/logging"
)

func ExampleWithSkipPaths() {
	// WithSkipPaths configures paths to skip logging.
	opt := logging.WithSkipPaths("/health", "/metrics")
	fmt.Println("option created:", opt != nil)
	// Output:
	// option created: true
}
