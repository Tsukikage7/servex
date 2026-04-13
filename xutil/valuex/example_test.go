package valuex_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/xutil/valuex"
)

func ExampleOf() {
	av := valuex.Of(42)
	fmt.Println(av.Val)
	fmt.Println(av.Err)
	// Output:
	// 42
	// <nil>
}
