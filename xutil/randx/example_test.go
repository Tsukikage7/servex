package randx_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/xutil/randx"
)

func ExampleRand_RandAlphanumeric() {
	r := randx.New()
	s := r.RandAlphanumeric(10)
	fmt.Println(len(s))
	// Output:
	// 10
}

func ExampleRand_RandDigits() {
	r := randx.New()
	s := r.RandDigits(6)
	fmt.Println(len(s))
	// Output:
	// 6
}
