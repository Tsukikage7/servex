package lrucache_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/collections/lrucache"
)

func ExampleNew() {
	cache := lrucache.New[string, int](3)
	fmt.Println(cache.Capacity())
	fmt.Println(cache.Len())
	// Output:
	// 3
	// 0
}

func ExampleLRUCache_Put() {
	cache := lrucache.New[string, int](3)

	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3)

	val, ok := cache.Get("a")
	fmt.Println(val, ok)
	fmt.Println(cache.Len())
	// Output:
	// 1 true
	// 3
}

func ExampleLRUCache_Get_eviction() {
	cache := lrucache.New[string, int](2)

	cache.Put("a", 1)
	cache.Put("b", 2)

	// 访问 "a" 使其变为最近使用.
	cache.Get("a")

	// 添加 "c" 触发淘汰，"b" 是最久未使用的，将被淘汰.
	cache.Put("c", 3)

	_, okA := cache.Get("a")
	_, okB := cache.Get("b")
	_, okC := cache.Get("c")
	fmt.Println("a exists:", okA)
	fmt.Println("b exists:", okB)
	fmt.Println("c exists:", okC)
	// Output:
	// a exists: true
	// b exists: false
	// c exists: true
}

func ExampleLRUCache_Keys() {
	cache := lrucache.New[string, int](3)

	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3)

	// Keys 按最近使用顺序返回.
	keys := cache.Keys()
	fmt.Println(keys)
	// Output: [c b a]
}
