package strx_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/xutil/strx"
)

func ExampleSplitName() {
	first, last := strx.SplitName("John Doe")
	fmt.Println(first)
	fmt.Println(last)
	// Output:
	// John
	// Doe
}

func ExampleIsEmpty() {
	fmt.Println(strx.IsEmpty(""))
	fmt.Println(strx.IsEmpty("   "))
	fmt.Println(strx.IsEmpty("hello"))
	// Output:
	// true
	// true
	// false
}
