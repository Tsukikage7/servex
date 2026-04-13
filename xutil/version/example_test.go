package version_test

import (
	"fmt"
	"runtime"

	"github.com/Tsukikage7/servex/v2/xutil/version"
)

func ExampleGet() {
	info := version.Get()
	fmt.Println(info.Version)
	fmt.Println(info.GoVersion == runtime.Version())
	// Output:
	// dev
	// true
}
