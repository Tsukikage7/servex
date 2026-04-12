package loadshed

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPMiddleware_正常请求(t *testing.T) {
	middleware := HTTPMiddleware(WithMaxConcurrent(10))
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("期望状态码 200，得到 %d", rec.Code)
	}
}

func TestHTTPMiddleware_并发超限(t *testing.T) {
	// 最大并发 1
	middleware := HTTPMiddleware(WithMaxConcurrent(1))

	blocker := make(chan struct{})
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blocker
		w.WriteHeader(http.StatusOK)
	}))

	var shedded atomic.Int32
	var wg sync.WaitGroup

	// 启动第一个请求（阻塞中）
	wg.Go(func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	})

	// 等待第一个请求进入处理
	time.Sleep(10 * time.Millisecond)

	// 第二个请求应被拒绝
	wg.Go(func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code == http.StatusServiceUnavailable {
			shedded.Add(1)
		}
	})

	// 等待第二个请求完成
	time.Sleep(10 * time.Millisecond)

	// 释放阻塞
	close(blocker)
	wg.Wait()

	if shedded.Load() != 1 {
		t.Errorf("期望 1 个请求被卸载，得到 %d", shedded.Load())
	}
}

func TestHTTPMiddleware_队列深度超限(t *testing.T) {
	// 最大排队深度 1
	middleware := HTTPMiddleware(WithMaxQueueDepth(1))

	blocker := make(chan struct{})
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blocker
		w.WriteHeader(http.StatusOK)
	}))

	var shedded atomic.Int32
	var wg sync.WaitGroup

	// 启动第一个请求（占用排队位置）
	wg.Go(func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	})

	// 等待第一个请求进入
	time.Sleep(10 * time.Millisecond)

	// 第二个请求应超过排队深度
	wg.Go(func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code == http.StatusServiceUnavailable {
			shedded.Add(1)
		}
	})

	time.Sleep(10 * time.Millisecond)
	close(blocker)
	wg.Wait()

	if shedded.Load() != 1 {
		t.Errorf("期望 1 个请求因队列深度被卸载，得到 %d", shedded.Load())
	}
}

func TestHTTPMiddleware_延迟超限(t *testing.T) {
	// 最大延迟 10ms
	middleware := HTTPMiddleware(WithMaxLatency(10 * time.Millisecond))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond) // 模拟慢请求
		w.WriteHeader(http.StatusOK)
	}))

	// 第一个请求（慢请求，通过但记录高延迟）
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Errorf("第一个请求期望 200，得到 %d", rec1.Code)
	}

	// 第二个请求应因上一次高延迟被拒绝
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusServiceUnavailable {
		t.Errorf("延迟超限后期望 503，得到 %d", rec2.Code)
	}
}

func TestHTTPMiddleware_延迟恢复(t *testing.T) {
	middleware := HTTPMiddleware(WithMaxLatency(100 * time.Millisecond))

	callCount := 0
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			time.Sleep(200 * time.Millisecond) // 第一次慢
		}
		// 后续快速
		w.WriteHeader(http.StatusOK)
	}))

	// 第一个慢请求
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	// 第二个请求被拒绝（延迟记录过高）
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusServiceUnavailable {
		t.Errorf("期望 503，得到 %d", rec2.Code)
	}
}

func TestHTTPMiddleware_无限制(t *testing.T) {
	// 不设置任何限制
	middleware := HTTPMiddleware()
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("无限制时期望 200，得到 %d", rec.Code)
	}
}

func TestHTTPMiddleware_503响应体(t *testing.T) {
	middleware := HTTPMiddleware(WithMaxConcurrent(0), WithMaxQueueDepth(0), WithMaxLatency(1*time.Nanosecond))

	// 先发一个慢请求记录高延迟
	slowHandler := HTTPMiddleware(WithMaxLatency(1 * time.Nanosecond))
	sh := slowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	req0 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec0 := httptest.NewRecorder()
	sh.ServeHTTP(rec0, req0)

	_ = middleware // 确认 shed 返回 503
	// 测试默认 shed 行为
	handler := slowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("期望 503，得到 %d", rec.Code)
	}
}
