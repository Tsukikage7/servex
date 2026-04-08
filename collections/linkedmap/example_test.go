package linkedmap_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/collections/linkedmap"
)

func ExampleNew() {
	m := linkedmap.New[string, int]()

	m.Put("c", 3)
	m.Put("a", 1)
	m.Put("b", 2)

	// 按插入顺序遍历.
	fmt.Println("keys:", m.Keys())
	fmt.Println("values:", m.Values())

	v, ok := m.Get("a")
	fmt.Println("get a:", v, ok)
	// Output:
	// keys: [c a b]
	// values: [3 1 2]
	// get a: 1 true
}

func ExampleLinkedMap_Remove() {
	m := linkedmap.New[string, int]()
	m.Put("x", 10)
	m.Put("y", 20)
	m.Put("z", 30)

	m.Remove("y")
	fmt.Println("keys:", m.Keys())
	fmt.Println("len:", m.Len())
	// Output:
	// keys: [x z]
	// len: 2
}
