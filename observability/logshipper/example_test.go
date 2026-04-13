package logshipper_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/observability/logshipper"
)

func ExampleEntry() {
	entry := logshipper.Entry{
		Level:   "info",
		Message: "server started",
	}
	fmt.Println(entry.Level)
	fmt.Println(entry.Message)
	// Output:
	// info
	// server started
}
