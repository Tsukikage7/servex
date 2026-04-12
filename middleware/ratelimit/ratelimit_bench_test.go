package ratelimit

import (
	"testing"
	"time"
)

func BenchmarkTokenBucket_Allow(b *testing.B) {
	tb := NewTokenBucket(1e9, 1e9) // very high rate to avoid blocking
	ctx := b.Context()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		tb.Allow(ctx)
	}
}

func BenchmarkTokenBucket_Allow_Parallel(b *testing.B) {
	tb := NewTokenBucket(1e9, 1e9)
	ctx := b.Context()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tb.Allow(ctx)
		}
	})
}

func BenchmarkSlidingWindow_Allow(b *testing.B) {
	sw := NewSlidingWindow(1<<30, time.Hour) // very high limit
	ctx := b.Context()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		sw.Allow(ctx)
	}
}

func BenchmarkSlidingWindow_Allow_Parallel(b *testing.B) {
	sw := NewSlidingWindow(1<<30, time.Hour)
	ctx := b.Context()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sw.Allow(ctx)
		}
	})
}
