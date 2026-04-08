package syncx_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/xutil/syncx"
)

func ExampleMap() {
	var m syncx.Map[string, int]
	m.Store("count", 42)

	val, ok := m.Load("count")
	fmt.Println(ok)
	fmt.Println(val)

	_, ok = m.Load("missing")
	fmt.Println(ok)
	// Output:
	// true
	// 42
	// false
}

func ExampleNewPool() {
	pool := syncx.NewPool(func() []byte {
		return make([]byte, 0, 1024)
	})
	buf := pool.Get()
	fmt.Println(cap(buf))
	pool.Put(buf)
	// Output:
	// 1024
}
