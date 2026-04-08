package ptrx_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/xutil/ptrx"
)

func ExampleToPtr() {
	p := ptrx.ToPtr(42)
	fmt.Println(*p)
	// Output:
	// 42
}

func ExampleValue() {
	p := ptrx.ToPtr("hello")
	fmt.Println(ptrx.Value(p))

	var nilPtr *string
	fmt.Println(ptrx.Value(nilPtr))
	// Output:
	// hello
	//
}
