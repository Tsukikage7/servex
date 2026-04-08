package lrucache

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func BenchmarkLRUCache_Get(b *testing.B) {
	cache := New[int, int](1000)
	for i := range 1000 {
		cache.Put(i, i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		cache.Get(42)
	}
}

func BenchmarkLRUCache_Put(b *testing.B) {
	cache := New[int, int](1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := b.Loop(); i; i = b.Loop() {
		cache.Put(b.N%1000, b.N)
	}
}

func BenchmarkLRUCache_Get_Miss(b *testing.B) {
	cache := New[int, int](1000)
	for i := range 1000 {
		cache.Put(i, i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		cache.Get(9999)
	}
}

func BenchmarkLRUCache_Put_Evict(b *testing.B) {
	cache := New[int, int](100)
	for i := range 100 {
		cache.Put(i, i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	i := 100
	for b.Loop() {
		cache.Put(i, i)
		i++
	}
}

func BenchmarkLRUCache_Concurrent_GetPut(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		cache := New[int, int](size)
		for i := range size {
			cache.Put(i, i)
		}
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			var counter atomic.Int64
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					i := int(counter.Add(1))
					if i%2 == 0 {
						cache.Get(i % size)
					} else {
						cache.Put(i%size, i)
					}
				}
			})
		})
	}
}
