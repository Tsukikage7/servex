package mapsx_test

import (
	"fmt"
	"sort"

	"github.com/Tsukikage7/servex/v2/collections/mapsx"
)

func ExampleMerge() {
	m1 := map[string]int{"a": 1, "b": 2}
	m2 := map[string]int{"b": 20, "c": 3}

	merged := mapsx.Merge(m1, m2)

	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Printf("%s: %d\n", k, merged[k])
	}
	// Output:
	// a: 1
	// b: 20
	// c: 3
}

func ExampleFilter() {
	m := map[string]int{"a": 1, "b": 2, "c": 3, "d": 4}

	filtered := mapsx.Filter(m, func(_ string, v int) bool {
		return v > 2
	})

	keys := make([]string, 0, len(filtered))
	for k := range filtered {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Printf("%s: %d\n", k, filtered[k])
	}
	// Output:
	// c: 3
	// d: 4
}

func ExampleEqual() {
	m1 := map[string]int{"a": 1, "b": 2}
	m2 := map[string]int{"a": 1, "b": 2}
	m3 := map[string]int{"a": 1, "b": 3}

	fmt.Println(mapsx.Equal(m1, m2))
	fmt.Println(mapsx.Equal(m1, m3))
	// Output:
	// true
	// false
}
