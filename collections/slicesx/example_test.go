package slicesx_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/collections/slicesx"
)

func ExampleMap() {
	nums := []int{1, 2, 3, 4}
	doubled := slicesx.Map(nums, func(n int) int { return n * 2 })
	fmt.Println(doubled)
	// Output:
	// [2 4 6 8]
}

func ExampleFilter() {
	nums := []int{1, 2, 3, 4, 5, 6}
	evens := slicesx.Filter(nums, func(n int) bool { return n%2 == 0 })
	fmt.Println(evens)
	// Output:
	// [2 4 6]
}

func ExampleReduce() {
	nums := []int{1, 2, 3, 4}
	sum := slicesx.Reduce(nums, 0, func(acc, n int) int { return acc + n })
	fmt.Println(sum)
	// Output:
	// 10
}

func ExampleUnique() {
	nums := []int{1, 2, 2, 3, 1, 4, 3}
	unique := slicesx.Unique(nums)
	fmt.Println(unique)
	// Output:
	// [1 2 3 4]
}

func ExampleChunk() {
	nums := []int{1, 2, 3, 4, 5}
	chunks := slicesx.Chunk(nums, 2)
	fmt.Println(chunks)
	// Output:
	// [[1 2] [3 4] [5]]
}
