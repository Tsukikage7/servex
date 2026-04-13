package semaphore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/Tsukikage7/servex/v2/storage/cache"
	"github.com/Tsukikage7/servex/v2/testx"
)

// newTestSemaphore 创建测试用的信号量.
func newTestSemaphore(size int64) (*Distributed, cache.Cache) {
	memCache, _ := cache.NewMemoryCache(nil, testx.NopLogger())
	counter := CacheCounter(memCache)
	return New(counter, "test-sem", size), memCache
}

func TestRedisSemaphore(t *testing.T) {
	sem, memCache := newTestSemaphore(3)
	defer memCache.Close()

	ctx := t.Context()

	t.Run("acquire and release", func(t *testing.T) {
		if err := sem.Acquire(ctx); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		available, _ := sem.Available(ctx)
		if available != 2 {
			t.Errorf("expected 2 available, got %d", available)
		}

		if err := sem.Release(ctx); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		available, _ = sem.Available(ctx)
		if available != 3 {
			t.Errorf("expected 3 available, got %d", available)
		}
	})

	t.Run("try acquire", func(t *testing.T) {
		// 获取所有许可
		for i := 0; i < 3; i++ {
			if !sem.TryAcquire(ctx) {
				t.Errorf("expected to acquire permit %d", i)
			}
		}

		// 第4个应该失败
		if sem.TryAcquire(ctx) {
			t.Error("expected to fail acquiring 4th permit")
		}

		// 释放所有
		for i := 0; i < 3; i++ {
			_ = sem.Release(ctx)
		}
	})

	t.Run("size", func(t *testing.T) {
		if sem.Size() != 3 {
			t.Errorf("expected size 3, got %d", sem.Size())
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		// 先占满
		for i := 0; i < 3; i++ {
			_ = sem.TryAcquire(ctx)
		}

		cancelCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		err := sem.Acquire(cancelCtx)
		if err != context.DeadlineExceeded {
			t.Errorf("expected DeadlineExceeded, got %v", err)
		}

		// 释放
		for i := 0; i < 3; i++ {
			_ = sem.Release(ctx)
		}
	})
}

func TestRedisConcurrency(t *testing.T) {
	sem, memCache := newTestSemaphore(5)
	defer memCache.Close()

	ctx := t.Context()
	var maxConcurrent atomic.Int32
	var current atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Go(func() {
			if err := sem.Acquire(ctx); err != nil {
				return
			}
			defer sem.Release(ctx)

			c := current.Add(1)
			for {
				max := maxConcurrent.Load()
				if c <= max || maxConcurrent.CompareAndSwap(max, c) {
					break
				}
			}

			time.Sleep(10 * time.Millisecond)
			current.Add(-1)
		})
	}

	wg.Wait()

	if maxConcurrent.Load() > 5 {
		t.Errorf("max concurrent exceeded limit: %d > 5", maxConcurrent.Load())
	}
}

func TestEndpointMiddleware(t *testing.T) {
	sem, memCache := newTestSemaphore(2)
	defer memCache.Close()

	var callCount atomic.Int32
	endpoint := func(ctx context.Context, request any) (any, error) {
		callCount.Add(1)
		time.Sleep(50 * time.Millisecond)
		return "ok", nil
	}

	wrapped := EndpointMiddleware(sem)(endpoint)

	ctx := t.Context()
	var wg sync.WaitGroup
	var errors atomic.Int32

	// 启动5个并发请求
	for i := 0; i < 5; i++ {
		wg.Go(func() {
			_, err := wrapped(ctx, nil)
			if err != nil {
				errors.Add(1)
			}
		})
	}

	wg.Wait()

	// 应该有3个失败（因为只允许2个并发）
	if errors.Load() != 3 {
		t.Errorf("expected 3 errors, got %d", errors.Load())
	}
}

func TestEndpointMiddlewareWithBlock(t *testing.T) {
	sem, memCache := newTestSemaphore(2)
	defer memCache.Close()

	var callCount atomic.Int32
	endpoint := func(ctx context.Context, request any) (any, error) {
		callCount.Add(1)
		time.Sleep(50 * time.Millisecond)
		return "ok", nil
	}

	wrapped := EndpointMiddleware(sem, WithBlock(true))(endpoint)

	ctx := t.Context()
	var wg sync.WaitGroup

	// 启动5个并发请求
	for i := 0; i < 5; i++ {
		wg.Go(func() {
			_, _ = wrapped(ctx, nil)
		})
	}

	wg.Wait()

	// 所有请求应该都成功（因为会阻塞等待）
	if callCount.Load() != 5 {
		t.Errorf("expected 5 calls, got %d", callCount.Load())
	}
}

func TestHTTPMiddleware(t *testing.T) {
	sem, memCache := newTestSemaphore(1)
	defer memCache.Close()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	wrapped := HTTPMiddleware(sem)(handler)

	// 第一个请求
	req1 := httptest.NewRequest("GET", "/", nil)
	rec1 := httptest.NewRecorder()

	// 第二个请求
	req2 := httptest.NewRequest("GET", "/", nil)
	rec2 := httptest.NewRecorder()

	var wg sync.WaitGroup
	wg.Add(2)

	// 使用 channel 确保第一个请求已开始处理后再发送第二个请求
	firstStarted := make(chan struct{})
	origHandler := handler
	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstStarted <- struct{}{}
		origHandler.ServeHTTP(w, r)
	})
	wrapped = HTTPMiddleware(sem)(handler)

	go func() {
		defer wg.Done()
		wrapped.ServeHTTP(rec1, req1)
	}()

	<-firstStarted // 等待第一个请求确实已开始

	go func() {
		defer wg.Done()
		wrapped.ServeHTTP(rec2, req2)
	}()

	wg.Wait()

	// 第一个应该成功
	if rec1.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec1.Code)
	}

	// 第二个应该被拒绝
	if rec2.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec2.Code)
	}
}

func TestUnaryServerInterceptor(t *testing.T) {
	sem, memCache := newTestSemaphore(1)
	defer memCache.Close()

	handler := func(ctx context.Context, req any) (any, error) {
		time.Sleep(100 * time.Millisecond)
		return "ok", nil
	}

	// 使用 channel 确保第一个请求已开始处理后再发送第二个请求
	firstStarted := make(chan struct{})
	origHandler := handler
	handler = func(ctx context.Context, req any) (any, error) {
		firstStarted <- struct{}{}
		return origHandler(ctx, req)
	}

	interceptor := UnaryServerInterceptor(sem)
	info := &grpc.UnaryServerInfo{FullMethod: "/test/Method"}

	var wg sync.WaitGroup
	var results [2]error

	wg.Add(2)

	go func() {
		defer wg.Done()
		_, results[0] = interceptor(t.Context(), nil, info, handler)
	}()

	<-firstStarted // 等待第一个请求确实已开始

	go func() {
		defer wg.Done()
		_, results[1] = interceptor(t.Context(), nil, info, handler)
	}()

	wg.Wait()

	// 一个成功，一个失败
	successCount := 0
	for _, err := range results {
		if err == nil {
			successCount++
		}
	}
	if successCount != 1 {
		t.Errorf("expected 1 success, got %d", successCount)
	}
}

func TestPanicOnInvalidSize(t *testing.T) {
	t.Run("nil counter", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic")
			}
		}()
		New(nil, "test", 10)
	})

	t.Run("zero size", func(t *testing.T) {
		memCache, _ := cache.NewMemoryCache(nil, testx.NopLogger())
		defer memCache.Close()

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic")
			}
		}()
		counter := CacheCounter(memCache)
		New(counter, "test", 0)
	})
}
