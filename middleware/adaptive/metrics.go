// Package adaptive 提供基于系统负载的自适应限流和降级中间件.
package adaptive

import (
	"math"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// slidingWindow 滑动窗口计数器，用于统计时间窗口内的指标.
type slidingWindow struct {
	mu       sync.Mutex
	window   time.Duration
	buckets  int
	bucketDu time.Duration
	current  int
	lastTick time.Time
	counts   []int64 // 每个桶的计数
}

// newSlidingWindow 创建滑动窗口计数器.
func newSlidingWindow(window time.Duration, buckets int) *slidingWindow {
	if buckets <= 0 {
		buckets = 10
	}
	return &slidingWindow{
		window:   window,
		buckets:  buckets,
		bucketDu: window / time.Duration(buckets),
		counts:   make([]int64, buckets),
		lastTick: time.Now(),
	}
}

// advance 推进窗口，清除过期桶.
func (sw *slidingWindow) advance(now time.Time) {
	elapsed := now.Sub(sw.lastTick)
	ticks := int(elapsed / sw.bucketDu)
	if ticks <= 0 {
		return
	}
	if ticks >= sw.buckets {
		for i := range sw.counts {
			sw.counts[i] = 0
		}
		sw.current = 0
	} else {
		for i := 0; i < ticks; i++ {
			sw.current = (sw.current + 1) % sw.buckets
			sw.counts[sw.current] = 0
		}
	}
	sw.lastTick = now
}

// add 向当前桶增加计数.
func (sw *slidingWindow) add(n int64) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.advance(time.Now())
	sw.counts[sw.current] += n
}

// sum 获取窗口内的总计数.
func (sw *slidingWindow) sum() int64 {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.advance(time.Now())
	var total int64
	for _, c := range sw.counts {
		total += c
	}
	return total
}

// latencyTracker 延迟追踪器，支持百分位统计.
type latencyTracker struct {
	mu      sync.Mutex
	window  time.Duration
	samples []latencySample
}

// latencySample 延迟样本.
type latencySample struct {
	ts time.Time
	d  time.Duration
}

// newLatencyTracker 创建延迟追踪器.
func newLatencyTracker(window time.Duration) *latencyTracker {
	return &latencyTracker{
		window:  window,
		samples: make([]latencySample, 0, 1024),
	}
}

// record 记录一个延迟样本.
func (lt *latencyTracker) record(d time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	now := time.Now()
	lt.cleanup(now)
	lt.samples = append(lt.samples, latencySample{ts: now, d: d})
}

// percentile 获取百分位延迟（如 0.99 表示 P99）.
func (lt *latencyTracker) percentile(p float64) time.Duration {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	lt.cleanup(time.Now())
	n := len(lt.samples)
	if n == 0 {
		return 0
	}
	// 复制并排序
	sorted := make([]time.Duration, n)
	for i, s := range lt.samples {
		sorted[i] = s.d
	}
	slices.Sort(sorted)
	idx := int(math.Ceil(p*float64(n))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}

// cleanup 清除过期样本.
func (lt *latencyTracker) cleanup(now time.Time) {
	cutoff := now.Add(-lt.window)
	i := 0
	for i < len(lt.samples) && lt.samples[i].ts.Before(cutoff) {
		i++
	}
	if i > 0 {
		lt.samples = lt.samples[i:]
	}
}

// errorRateTracker 错误率追踪器.
type errorRateTracker struct {
	success *slidingWindow
	failure *slidingWindow
	total   atomic.Int64
	dropped atomic.Int64
}

// newErrorRateTracker 创建错误率追踪器.
func newErrorRateTracker(window time.Duration) *errorRateTracker {
	return &errorRateTracker{
		success: newSlidingWindow(window, 10),
		failure: newSlidingWindow(window, 10),
	}
}

// recordSuccess 记录成功事件.
func (e *errorRateTracker) recordSuccess() {
	e.success.add(1)
	e.total.Add(1)
}

// recordError 记录错误事件.
func (e *errorRateTracker) recordError() {
	e.failure.add(1)
	e.total.Add(1)
}

// recordDrop 记录被丢弃的请求.
func (e *errorRateTracker) recordDrop() {
	e.dropped.Add(1)
}

// errorRate 获取当前错误率.
func (e *errorRateTracker) errorRate() float64 {
	s := e.success.sum()
	f := e.failure.sum()
	total := s + f
	if total == 0 {
		return 0
	}
	return float64(f) / float64(total)
}
