package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPMiddleware(t *testing.T) {
	t.Run("允许请求", func(t *testing.T) {
		limiter := NewTokenBucket(10, 10)
		middleware := HTTPMiddleware(limiter)

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("期望 200，得到 %d", rec.Code)
		}
	})

	t.Run("拒绝请求", func(t *testing.T) {
		limiter := NewTokenBucket(1, 1)
		middleware := HTTPMiddleware(limiter)

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// 第一个请求通过
		req1 := httptest.NewRequest(http.MethodGet, "/", nil)
		rec1 := httptest.NewRecorder()
		handler.ServeHTTP(rec1, req1)

		if rec1.Code != http.StatusOK {
			t.Errorf("第一个请求期望 200，得到 %d", rec1.Code)
		}

		// 第二个请求被限流
		req2 := httptest.NewRequest(http.MethodGet, "/", nil)
		rec2 := httptest.NewRecorder()
		handler.ServeHTTP(rec2, req2)

		if rec2.Code != http.StatusTooManyRequests {
			t.Errorf("第二个请求期望 429，得到 %d", rec2.Code)
		}
	})
}

func TestKeyedHTTPMiddleware(t *testing.T) {
	t.Run("基于IP限流", func(t *testing.T) {
		limiters := make(map[string]*TokenBucket)

		getLimiter := func(key string) Limiter {
			if l, ok := limiters[key]; ok {
				return l
			}
			l := NewTokenBucket(1, 1)
			limiters[key] = l
			return l
		}

		middleware := KeyedHTTPMiddleware(IPKeyFunc(), getLimiter)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// IP1 第一个请求通过
		req1 := httptest.NewRequest(http.MethodGet, "/", nil)
		req1.RemoteAddr = "192.168.1.1:12345"
		rec1 := httptest.NewRecorder()
		handler.ServeHTTP(rec1, req1)

		if rec1.Code != http.StatusOK {
			t.Errorf("IP1 第一个请求期望 200，得到 %d", rec1.Code)
		}

		// IP1 第二个请求被限流
		req2 := httptest.NewRequest(http.MethodGet, "/", nil)
		req2.RemoteAddr = "192.168.1.1:12345"
		rec2 := httptest.NewRecorder()
		handler.ServeHTTP(rec2, req2)

		if rec2.Code != http.StatusTooManyRequests {
			t.Errorf("IP1 第二个请求期望 429，得到 %d", rec2.Code)
		}

		// IP2 第一个请求通过独立限流
		req3 := httptest.NewRequest(http.MethodGet, "/", nil)
		req3.RemoteAddr = "192.168.1.2:12345"
		rec3 := httptest.NewRecorder()
		handler.ServeHTTP(rec3, req3)

		if rec3.Code != http.StatusOK {
			t.Errorf("IP2 第一个请求期望 200，得到 %d", rec3.Code)
		}
	})
}

func TestIPKeyFunc(t *testing.T) {
	keyFunc := IPKeyFunc()

	t.Run("使用X-Forwarded-For", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		req.RemoteAddr = "192.168.1.1:12345"

		key := keyFunc(req)
		if key != "10.0.0.1" {
			t.Errorf("期望 10.0.0.1，得到 %s", key)
		}
	})

	t.Run("使用X-Real-IP", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Real-IP", "10.0.0.2")
		req.RemoteAddr = "192.168.1.1:12345"

		key := keyFunc(req)
		if key != "10.0.0.2" {
			t.Errorf("期望 10.0.0.2，得到 %s", key)
		}
	})

	t.Run("使用RemoteAddr", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"

		key := keyFunc(req)
		if key != "192.168.1.1:12345" {
			t.Errorf("期望 192.168.1.1:12345，得到 %s", key)
		}
	})
}

func TestPathKeyFunc(t *testing.T) {
	keyFunc := PathKeyFunc()

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	key := keyFunc(req)

	if key != "/api/users" {
		t.Errorf("期望 /api/users，得到 %s", key)
	}
}

func TestCompositeKeyFunc(t *testing.T) {
	keyFunc := CompositeKeyFunc(IPKeyFunc(), PathKeyFunc())

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	key := keyFunc(req)
	expected := "192.168.1.1:12345:/api/users"

	if key != expected {
		t.Errorf("期望 %s，得到 %s", expected, key)
	}
}
