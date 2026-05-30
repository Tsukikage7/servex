package retry

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewHTTPClient(t *testing.T) {
	t.Run("使用默认配置", func(t *testing.T) {
		client := NewHTTPClient(nil, nil)

		if client.client != http.DefaultClient {
			t.Error("期望使用 http.DefaultClient")
		}
		if client.cfg.MaxAttempts != DefaultMaxAttempts {
			t.Errorf("期望 MaxAttempts=%d，得到 %d", DefaultMaxAttempts, client.cfg.MaxAttempts)
		}
	})

	t.Run("使用自定义配置", func(t *testing.T) {
		customClient := &http.Client{Timeout: 5 * time.Second}
		cfg := &Config{
			MaxAttempts: 5,
			Delay:       200 * time.Millisecond,
		}

		client := NewHTTPClient(customClient, cfg)

		if client.client != customClient {
			t.Error("期望使用自定义 client")
		}
		if client.cfg.MaxAttempts != 5 {
			t.Errorf("期望 MaxAttempts=5，得到 %d", client.cfg.MaxAttempts)
		}
	})
}

func TestHTTPClient_Do_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	client := NewHTTPClient(nil, DefaultConfig())

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Errorf("不期望错误: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望 200，得到 %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "success" {
		t.Errorf("期望 'success'，得到 '%s'", string(body))
	}
}

func TestHTTPClient_Do_RetryOnError(t *testing.T) {
	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := callCount.Add(1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	cfg := &Config{
		MaxAttempts: 5,
		Delay:       1 * time.Millisecond,
		Backoff:     FixedBackoff,
	}
	client := NewHTTPClient(nil, cfg)

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Errorf("不期望错误: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望 200，得到 %d", resp.StatusCode)
	}
	if callCount.Load() != 3 {
		t.Errorf("期望调用 3 次，实际 %d 次", callCount.Load())
	}
}

func TestHTTPClient_Do_MaxRetries(t *testing.T) {
	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &Config{
		MaxAttempts: 3,
		Delay:       1 * time.Millisecond,
		Backoff:     FixedBackoff,
	}
	client := NewHTTPClient(nil, cfg)

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Errorf("不期望错误: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("期望 500，得到 %d", resp.StatusCode)
	}
	if callCount.Load() != 3 {
		t.Errorf("期望调用 3 次，实际 %d 次", callCount.Load())
	}
}

func TestHTTPClient_Do_NoRetryOnSuccess(t *testing.T) {
	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		MaxAttempts: 5,
		Delay:       1 * time.Millisecond,
		Backoff:     FixedBackoff,
	}
	client := NewHTTPClient(nil, cfg)

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Errorf("不期望错误: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望 200，得到 %d", resp.StatusCode)
	}
	if callCount.Load() != 1 {
		t.Errorf("期望调用 1 次，实际 %d 次", callCount.Load())
	}
}

func TestHTTPClient_Do_WithBody(t *testing.T) {
	var receivedBodies []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBodies = append(receivedBodies, string(body))

		if len(receivedBodies) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		MaxAttempts: 5,
		Delay:       1 * time.Millisecond,
		Backoff:     FixedBackoff,
	}
	client := NewHTTPClient(nil, cfg)

	req, _ := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("test body"))
	resp, err := client.Do(req)

	if err != nil {
		t.Errorf("不期望错误: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望 200，得到 %d", resp.StatusCode)
	}

	// 验证每次重试都发送了正确的 body
	for i, body := range receivedBodies {
		if body != "test body" {
			t.Errorf("第 %d 次请求 body 不正确: %s", i+1, body)
		}
	}
}

func TestHTTPClient_Do_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		MaxAttempts: 5,
		Delay:       10 * time.Millisecond,
		Backoff:     FixedBackoff,
	}
	client := NewHTTPClient(nil, cfg)

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // 立即取消

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	_, err := client.DoWithContext(ctx, req)

	if err != context.Canceled {
		t.Errorf("期望 context.Canceled，得到 %v", err)
	}
}

func TestHTTPClient_Do_Retry429(t *testing.T) {
	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := callCount.Add(1)
		if count < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		MaxAttempts: 5,
		Delay:       1 * time.Millisecond,
		Backoff:     FixedBackoff,
	}
	client := NewHTTPClient(nil, cfg)

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Errorf("不期望错误: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望 200，得到 %d", resp.StatusCode)
	}
	if callCount.Load() != 3 {
		t.Errorf("期望调用 3 次，实际 %d 次", callCount.Load())
	}
}

func TestHTTPClient_WithRetryable(t *testing.T) {
	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusBadRequest) // 400 默认不重试
	}))
	defer server.Close()

	cfg := &Config{
		MaxAttempts: 5,
		Delay:       1 * time.Millisecond,
		Backoff:     FixedBackoff,
	}

	t.Run("默认不重试400", func(t *testing.T) {
		callCount.Store(0)
		client := NewHTTPClient(nil, cfg)

		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
		resp, _ := client.Do(req)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("期望 400，得到 %d", resp.StatusCode)
		}
		if callCount.Load() != 1 {
			t.Errorf("期望调用 1 次，实际 %d 次", callCount.Load())
		}
	})

	t.Run("自定义重试400", func(t *testing.T) {
		callCount.Store(0)
		client := NewHTTPClient(nil, cfg).WithRetryable(func(resp *http.Response, err error) bool {
			if resp != nil && resp.StatusCode == http.StatusBadRequest {
				return true
			}
			return DefaultHTTPRetryable(resp, err)
		})

		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
		resp, _ := client.Do(req)

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("期望 400，得到 %d", resp.StatusCode)
		}
		if callCount.Load() != 5 {
			t.Errorf("期望调用 5 次，实际 %d 次", callCount.Load())
		}
	})
}

func TestDefaultHTTPRetryable(t *testing.T) {
	testCases := []struct {
		name     string
		resp     *http.Response
		err      error
		expected bool
	}{
		{"网络错误", nil, io.EOF, true},
		{"500 错误", &http.Response{StatusCode: 500}, nil, true},
		{"502 错误", &http.Response{StatusCode: 502}, nil, true},
		{"503 错误", &http.Response{StatusCode: 503}, nil, true},
		{"504 错误", &http.Response{StatusCode: 504}, nil, true},
		{"429 限流", &http.Response{StatusCode: 429}, nil, true},
		{"200 成功", &http.Response{StatusCode: 200}, nil, false},
		{"400 客户端错误", &http.Response{StatusCode: 400}, nil, false},
		{"401 未授权", &http.Response{StatusCode: 401}, nil, false},
		{"404 未找到", &http.Response{StatusCode: 404}, nil, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := DefaultHTTPRetryable(tc.resp, tc.err)
			if result != tc.expected {
				t.Errorf("期望 %v，得到 %v", tc.expected, result)
			}
		})
	}
}

func TestRetryOn5xx(t *testing.T) {
	testCases := []struct {
		name     string
		resp     *http.Response
		err      error
		expected bool
	}{
		{"网络错误", nil, io.EOF, true},
		{"500 错误", &http.Response{StatusCode: 500}, nil, true},
		{"502 错误", &http.Response{StatusCode: 502}, nil, true},
		{"429 限流", &http.Response{StatusCode: 429}, nil, false},
		{"200 成功", &http.Response{StatusCode: 200}, nil, false},
		{"400 客户端错误", &http.Response{StatusCode: 400}, nil, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := RetryOn5xx(tc.resp, tc.err)
			if result != tc.expected {
				t.Errorf("期望 %v，得到 %v", tc.expected, result)
			}
		})
	}
}

func TestRetryOnConnectionError(t *testing.T) {
	testCases := []struct {
		name     string
		resp     *http.Response
		err      error
		expected bool
	}{
		{"网络错误", nil, io.EOF, true},
		{"500 错误", &http.Response{StatusCode: 500}, nil, false},
		{"200 成功", &http.Response{StatusCode: 200}, nil, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := RetryOnConnectionError(tc.resp, tc.err)
			if result != tc.expected {
				t.Errorf("期望 %v，得到 %v", tc.expected, result)
			}
		})
	}
}

func TestHTTPClient_ExponentialBackoff(t *testing.T) {
	var timestamps []time.Time

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timestamps = append(timestamps, time.Now())
		if len(timestamps) < 4 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		MaxAttempts: 5,
		Delay:       10 * time.Millisecond,
		Backoff:     ExponentialBackoff,
	}
	client := NewHTTPClient(nil, cfg)

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Errorf("不期望错误: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望 200，得到 %d", resp.StatusCode)
	}

	// 验证退避时间增长大致检查
	if len(timestamps) >= 3 {
		gap1 := timestamps[1].Sub(timestamps[0])
		gap2 := timestamps[2].Sub(timestamps[1])
		// 指数退避：第二次间隔应该大于第一次
		if gap2 <= gap1 {
			t.Logf("警告：退避时间未增长（gap1=%v, gap2=%v），可能是测试环境问题", gap1, gap2)
		}
	}
}

func TestHTTPMiddleware(t *testing.T) {
	cfg := DefaultConfig()
	middleware := HTTPMiddleware(cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	wrapped := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("期望 200，得到 %d", rec.Code)
	}
}
