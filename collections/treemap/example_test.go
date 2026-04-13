package treemap_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/collections/treemap"
)

func ExampleNewOrdered() {
	tm := treemap.NewOrdered[int, string]()

	tm.Put(3, "three")
	tm.Put(1, "one")
	tm.Put(2, "two")

	fmt.Println("keys:", tm.Keys())
	fmt.Println("values:", tm.Values())
	fmt.Println("len:", tm.Len())

	v, ok := tm.Get(2)
	fmt.Println("get 2:", v, ok)
	// Output:
	// keys: [1 2 3]
	// values: [one two three]
	// len: 3
	// get 2: two true
}

func ExampleTreeMap_FirstKey() {
	tm := treemap.NewOrdered[string, int]()
	tm.Put("banana", 2)
	tm.Put("apple", 1)
	tm.Put("cherry", 3)

	first, _ := tm.FirstKey()
	last, _ := tm.LastKey()
	fmt.Println("first:", first)
	fmt.Println("last:", last)
	// Output:
	// first: apple
	// last: cherry
}

func ExampleTreeMap_Remove() {
	tm := treemap.NewOrdered[int, string]()
	tm.Put(1, "a")
	tm.Put(2, "b")
	tm.Put(3, "c")

	removed, ok := tm.Remove(2)
	fmt.Println("removed:", removed, ok)
	fmt.Println("keys:", tm.Keys())
	// Output:
	// removed: b true
	// keys: [1 3]
}
