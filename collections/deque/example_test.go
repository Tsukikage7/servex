package deque_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/collections/deque"
)

func ExampleNew() {
	dq := deque.New[int]()

	dq.PushBack(1)
	dq.PushBack(2)
	dq.PushFront(0)

	fmt.Println("len:", dq.Len())
	fmt.Println(dq.ToSlice())
	// Output:
	// len: 3
	// [0 1 2]
}

func ExampleDeque_PopFront() {
	dq := deque.From([]string{"a", "b", "c"})

	front, _ := dq.PopFront()
	back, _ := dq.PopBack()
	fmt.Println("front:", front)
	fmt.Println("back:", back)
	fmt.Println("remaining:", dq.Len())
	// Output:
	// front: a
	// back: c
	// remaining: 1
}

func ExampleDeque_Reverse() {
	dq := deque.From([]int{1, 2, 3, 4, 5})
	dq.Reverse()
	fmt.Println(dq.ToSlice())
	// Output:
	// [5 4 3 2 1]
}
