package syncx

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func BenchmarkMap_Load(b *testing.B) {
	var m Map[string, int]
	m.Store("key", 42)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m.Load("key")
	}
}

func BenchmarkMap_Store(b *testing.B) {
	var m Map[string, int]
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m.Store("key", 42)
	}
}

func BenchmarkMap_LoadOrStore(b *testing.B) {
	var m Map[string, int]
	m.Store("key", 42)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m.LoadOrStore("key", 0)
	}
}

func BenchmarkMap_Load_Parallel(b *testing.B) {
	var m Map[string, int]
	for i := range 1000 {
		m.Store(fmt.Sprintf("key_%d", i), i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			m.Load(fmt.Sprintf("key_%d", i%1000))
			i++
		}
	})
}

func BenchmarkMap_Store_Parallel(b *testing.B) {
	var m Map[string, int]
	var counter atomic.Int64
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := counter.Add(1)
			m.Store(fmt.Sprintf("key_%d", i%1000), int(i))
		}
	})
}

func BenchmarkMap_LoadOrStore_Parallel(b *testing.B) {
	var m Map[string, int]
	for i := range 1000 {
		m.Store(fmt.Sprintf("key_%d", i), i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			m.LoadOrStore(fmt.Sprintf("key_%d", i%1000), 0)
			i++
		}
	})
}
