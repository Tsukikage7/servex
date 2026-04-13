package pagination_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/xutil/pagination"
)

func ExampleNew() {
	p := pagination.New(2, 10)
	fmt.Println(p.Page)
	fmt.Println(p.PageSize)
	fmt.Println(p.Offset())
	fmt.Println(p.Limit())
	// Output:
	// 2
	// 10
	// 10
	// 10
}

func ExampleNew_defaults() {
	p := pagination.New(0, 0)
	fmt.Println(p.Page)
	fmt.Println(p.PageSize)
	// Output:
	// 1
	// 20
}
