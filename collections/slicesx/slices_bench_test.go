package slicesx

import (
	"fmt"
	"testing"
)

// --- helpers ---

func makeIntSlice(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i
	}
	return s
}

func makeNestedSlice(n int) [][]int {
	chunks := n / 10
	if chunks == 0 {
		chunks = 1
	}
	s := make([][]int, chunks)
	per := n / chunks
	for i := range s {
		s[i] = make([]int, per)
		for j := range s[i] {
			s[i][j] = i*per + j
		}
	}
	return s
}

// --- Map ---

func BenchmarkMap(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		data := makeIntSlice(size)
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				Map(data, func(n int) int { return n * 2 })
			}
		})
	}
}

// --- Filter ---

func BenchmarkFilter(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		data := makeIntSlice(size)
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				Filter(data, func(n int) bool { return n%2 == 0 })
			}
		})
	}
}

// --- Reduce ---

func BenchmarkReduce(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		data := makeIntSlice(size)
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				Reduce(data, 0, func(acc, n int) int { return acc + n })
			}
		})
	}
}

// --- Unique ---

func BenchmarkUnique(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		// half duplicates
		data := makeIntSlice(size)
		for i := size / 2; i < size; i++ {
			data[i] = data[i-size/2]
		}
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				Unique(data)
			}
		})
	}
}

// --- Contains ---

func BenchmarkContains(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		data := makeIntSlice(size)
		target := size - 1 // worst case: last element
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				Contains(data, target)
			}
		})
	}
}

// --- Flatten ---

func BenchmarkFlatten(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		data := makeNestedSlice(size)
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				Flatten(data)
			}
		})
	}
}

// --- GroupBy ---

func BenchmarkGroupBy(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		data := makeIntSlice(size)
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				GroupBy(data, func(n int) int { return n % 10 })
			}
		})
	}
}
