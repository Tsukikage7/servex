package mapsx

import (
	"fmt"
	"testing"
)

func makeStringIntMap(n int) map[string]int {
	m := make(map[string]int, n)
	for i := range n {
		m[fmt.Sprintf("key_%d", i)] = i
	}
	return m
}

func BenchmarkKeys(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		m := makeStringIntMap(size)
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				Keys(m)
			}
		})
	}
}

func BenchmarkValues(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		m := makeStringIntMap(size)
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				Values(m)
			}
		})
	}
}

func BenchmarkMerge(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		m1 := makeStringIntMap(size)
		m2 := makeStringIntMap(size)
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				Merge(m1, m2)
			}
		})
	}
}

func BenchmarkFilter_Map(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		m := makeStringIntMap(size)
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				Filter(m, func(_ string, v int) bool { return v%2 == 0 })
			}
		})
	}
}

func BenchmarkMapValues(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		m := makeStringIntMap(size)
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				MapValues(m, func(v int) int { return v * 2 })
			}
		})
	}
}
