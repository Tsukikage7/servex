package treeset_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/collections/treeset"
)

func ExampleNewOrdered() {
	ts := treeset.NewOrdered[int]()

	ts.Add(5, 3, 1, 4, 2)

	fmt.Println("sorted:", ts.ToSlice())
	fmt.Println("contains 3:", ts.Contains(3))
	fmt.Println("contains 6:", ts.Contains(6))
	fmt.Println("len:", ts.Len())
	// Output:
	// sorted: [1 2 3 4 5]
	// contains 3: true
	// contains 6: false
	// len: 5
}

func ExampleTreeSet_First() {
	ts := treeset.FromSlice([]string{"cherry", "apple", "banana"})

	first, _ := ts.First()
	last, _ := ts.Last()
	fmt.Println("first:", first)
	fmt.Println("last:", last)
	// Output:
	// first: apple
	// last: cherry
}

func ExampleTreeSet_Intersection() {
	a := treeset.FromSlice([]int{1, 2, 3, 4})
	b := treeset.FromSlice([]int{3, 4, 5, 6})

	inter := a.Intersection(b)
	fmt.Println("intersection:", inter.ToSlice())

	diff := a.Difference(b)
	fmt.Println("difference:", diff.ToSlice())
	// Output:
	// intersection: [3 4]
	// difference: [1 2]
}
