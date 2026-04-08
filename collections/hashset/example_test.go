package hashset_test

import (
	"fmt"
	"sort"

	"github.com/Tsukikage7/servex/collections/hashset"
)

func ExampleNew() {
	s := hashset.New(1, 2, 3, 4, 5)

	fmt.Println("contains 3:", s.Contains(3))
	fmt.Println("contains 6:", s.Contains(6))
	fmt.Println("len:", s.Len())

	s.Remove(3)
	fmt.Println("after remove 3:", s.Contains(3))
	// Output:
	// contains 3: true
	// contains 6: false
	// len: 5
	// after remove 3: false
}

func ExampleHashSet_Intersection() {
	a := hashset.New(1, 2, 3, 4)
	b := hashset.New(3, 4, 5, 6)

	inter := a.Intersection(b)
	result := inter.ToSlice()
	sort.Ints(result)
	fmt.Println(result)
	// Output:
	// [3 4]
}

func ExampleHashSet_Union() {
	a := hashset.New("go", "rust")
	b := hashset.New("rust", "python")

	union := a.Union(b)
	result := union.ToSlice()
	sort.Strings(result)
	fmt.Println(result)
	// Output:
	// [go python rust]
}
