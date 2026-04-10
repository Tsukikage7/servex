package fallback

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPMiddleware_正常响应(t *testing.T) {
	middleware := HTTPMiddleware()
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("期望状态码 200，得到 %d", rec.Code)
	}
	if rec.Body.String() != "success" {
		t.Errorf("期望响应体 %q，得到 %q", "success", rec.Body.String())
	}
}

func TestHTTPMiddleware_5xx触发降级(t *testing.T) {
	fallbackCalled := false
	middleware := HTTPMiddleware(
		WithFallbackFunc(func(w http.ResponseWriter, r *http.Request) {
			fallbackCalled = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("fallback response"))
		}),
	)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !fallbackCalled {
		t.Error("期望降级处理器被调用")
	}
	if rec.Body.String() != "fallback response" {
		t.Errorf("期望降级响应体 %q，得到 %q", "fallback response", rec.Body.String())
	}
}

func TestHTTPMiddleware_panic触发降级(t *testing.T) {
	fallbackCalled := false
	middleware := HTTPMiddleware(
		WithFallbackFunc(func(w http.ResponseWriter, r *http.Request) {
			fallbackCalled = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("recovered"))
		}),
	)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !fallbackCalled {
		t.Error("期望 panic 后降级处理器被调用")
	}
}

func TestHTTPMiddleware_默认降级响应(t *testing.T) {
	// 不设置降级处理器，使用默认 503 响应
	middleware := HTTPMiddleware()
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("期望默认降级状态码 503，得到 %d", rec.Code)
	}
}

func TestHTTPMiddleware_panic默认降级(t *testing.T) {
	middleware := HTTPMiddleware()
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("期望 panic 后默认降级 503，得到 %d", rec.Code)
	}
}

func TestHTTPMiddleware_4xx不触发降级(t *testing.T) {
	fallbackCalled := false
	middleware := HTTPMiddleware(
		WithFallbackFunc(func(w http.ResponseWriter, r *http.Request) {
			fallbackCalled = true
		}),
	)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if fallbackCalled {
		t.Error("4xx 不应触发降级")
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("期望状态码 404，得到 %d", rec.Code)
	}
}

func TestHTTPMiddleware_WithFallbackHandler(t *testing.T) {
	fallbackHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("handler fallback"))
	})

	middleware := HTTPMiddleware(
		WithFallbackHandler(fallbackHandler),
	)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Body.String() != "handler fallback" {
		t.Errorf("期望 %q，得到 %q", "handler fallback", rec.Body.String())
	}
}
