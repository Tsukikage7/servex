package sorting_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/xutil/sorting"
)

func ExampleNew() {
	s := sorting.New("created_time:desc,name:asc")
	fmt.Println(s.String())
	fmt.Println(s.IsEmpty())
	// Output:
	// created_time desc, name asc
	// false
}

func ExampleSorting_IsEmpty() {
	s := sorting.New("")
	fmt.Println(s.IsEmpty())
	// Output:
	// true
}
