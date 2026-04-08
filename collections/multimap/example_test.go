package multimap_test

import (
	"fmt"
	"sort"

	"github.com/Tsukikage7/servex/collections/multimap"
)

func ExampleNew() {
	mm := multimap.New[string, int]()

	mm.Put("colors", 1)
	mm.Put("colors", 2)
	mm.Put("colors", 3)
	mm.Put("sizes", 10)
	mm.Put("sizes", 20)

	fmt.Println("colors:", mm.Get("colors"))
	fmt.Println("sizes:", mm.Get("sizes"))
	fmt.Println("total values:", mm.Len())
	fmt.Println("total keys:", mm.KeyLen())
	// Output:
	// colors: [1 2 3]
	// sizes: [10 20]
	// total values: 5
	// total keys: 2
}

func ExampleMultiMap_Remove() {
	mm := multimap.New[string, string]()
	mm.PutAll("tags", "go", "rust", "python")

	keys := mm.Keys()
	sort.Strings(keys)
	fmt.Println("keys:", keys)

	mm.Remove("tags")
	fmt.Println("after remove:", mm.ContainsKey("tags"))
	// Output:
	// keys: [tags]
	// after remove: false
}
