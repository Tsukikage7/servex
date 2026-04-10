package secure

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTTPMiddleware_DefaultConfig(t *testing.T) {
	handler := HTTPMiddleware(DefaultConfig())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https") // 模拟 HTTPS 请求
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	assert.Contains(t, w.Header().Get("Strict-Transport-Security"), "max-age=31536000")
	assert.Contains(t, w.Header().Get("Strict-Transport-Security"), "includeSubDomains")
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
}

func TestHTTPMiddleware_NilConfig(t *testing.T) {
	handler := HTTPMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
}

func TestHTTPMiddleware_XFrameOptions(t *testing.T) {
	cfg := DefaultConfig()
	cfg.XFrameOptions = "SAMEORIGIN"

	handler := HTTPMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "SAMEORIGIN", w.Header().Get("X-Frame-Options"))
}

func TestHTTPMiddleware_ContentTypeNosniff(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ContentTypeNosniff = false

	handler := HTTPMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("X-Content-Type-Options"))
}

func TestHTTPMiddleware_CSP(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ContentSecurityPolicy = "default-src 'self'"

	handler := HTTPMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "default-src 'self'", w.Header().Get("Content-Security-Policy"))
}

func TestHTTPMiddleware_PermissionsPolicy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PermissionsPolicy = "camera=(), microphone=()"

	handler := HTTPMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "camera=(), microphone=()", w.Header().Get("Permissions-Policy"))
}

func TestHTTPMiddleware_DevelopmentModeSkipsHSTS(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IsDevelopment = true

	handler := HTTPMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Strict-Transport-Security"))
	// 其他安全头仍然存在
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
}

func TestHTTPMiddleware_HSTSPreload(t *testing.T) {
	cfg := DefaultConfig()
	cfg.HSTSPreload = true

	handler := HTTPMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https") // 模拟 HTTPS 请求
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	hsts := w.Header().Get("Strict-Transport-Security")
	assert.Contains(t, hsts, "preload")
	assert.Contains(t, hsts, "includeSubDomains")
}

func TestHTTPMiddleware_HSTSDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.HSTSMaxAge = 0

	handler := HTTPMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Strict-Transport-Security"))
}

func TestHTTPMiddleware_HSTSNotSetOnHTTP(t *testing.T) {
	// HSTS 不应在 HTTP 请求上设置
	handler := HTTPMiddleware(DefaultConfig())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Strict-Transport-Security"))
}

func TestHTTPMiddleware_EmptyXFrameOptions(t *testing.T) {
	cfg := DefaultConfig()
	cfg.XFrameOptions = ""

	handler := HTTPMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("X-Frame-Options"))
}
