package iox_test

import (
	"fmt"
	"strings"

	"github.com/Tsukikage7/servex/v2/xutil/iox"
)

func ExampleReadLines() {
	r := strings.NewReader("line1\nline2\nline3")
	lines, err := iox.ReadLines(r)
	fmt.Println(err)
	fmt.Println(lines)
	// Output:
	// <nil>
	// [line1 line2 line3]
}

func ExampleReadString() {
	r := strings.NewReader("hello world")
	s, err := iox.ReadString(r)
	fmt.Println(err)
	fmt.Println(s)
	// Output:
	// <nil>
	// hello world
}
