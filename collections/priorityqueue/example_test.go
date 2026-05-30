package priorityqueue_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/collections/priorityqueue"
)

func ExampleNewMin() {
	pq := priorityqueue.NewMin[int]()

	pq.Push(5, 3, 1, 4, 2)

	fmt.Println("len:", pq.Len())

	v1, _ := pq.Pop()
	v2, _ := pq.Pop()
	v3, _ := pq.Pop()
	fmt.Println(v1, v2, v3)
	// Output:
	// len: 5
	// 1 2 3
}

func ExampleNewMax() {
	pq := priorityqueue.NewMax[int]()

	pq.Push(5, 3, 1, 4, 2)

	v1, _ := pq.Pop()
	v2, _ := pq.Pop()
	fmt.Println(v1, v2)
	// Output:
	// 5 4
}

func ExampleNew() {
	// 自定义优先级: 按字符串长度短的优先.
	pq := priorityqueue.New(func(a, b string) bool {
		return len(a) < len(b)
	})

	pq.Push("hello", "hi", "hey")

	v, _ := pq.Pop()
	fmt.Println(v)
	// Output:
	// hi
}
