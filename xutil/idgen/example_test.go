package idgen_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/xutil/idgen"
)

func ExampleUUID() {
	id := idgen.UUID()
	// UUID v4 format: 8-4-4-4-12 hex characters.
	fmt.Println(len(id))
	// Output:
	// 36
}

func ExampleNanoID() {
	id := idgen.NanoID()
	// Default NanoID is 21 characters.
	fmt.Println(len(id))
	// Output:
	// 21
}
