package version

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPMiddleware_路径前缀(t *testing.T) {
	middleware := HTTPMiddleware()

	tests := []struct {
		name        string
		path        string
		wantVersion string
		wantPath    string
	}{
		{"v1 版本", "/v1/users", "v1", "/users"},
		{"v2 版本", "/v2/orders", "v2", "/orders"},
		{"v10 版本", "/v10/items", "v10", "/items"},
		{"无版本", "/users", "", "/users"},
		{"版本根路径", "/v1/", "v1", "/"},
		{"版本无尾斜杠", "/v1", "v1", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotVersion string
			var gotPath string

			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotVersion = FromContext(r.Context())
				gotPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if gotVersion != tt.wantVersion {
				t.Errorf("版本: 期望 %q，得到 %q", tt.wantVersion, gotVersion)
			}
			if gotPath != tt.wantPath {
				t.Errorf("路径: 期望 %q，得到 %q", tt.wantPath, gotPath)
			}
		})
	}
}

func TestHTTPMiddleware_请求头(t *testing.T) {
	middleware := HTTPMiddleware(WithPathPrefix(false))

	tests := []struct {
		name        string
		headerName  string
		headerValue string
		wantVersion string
	}{
		{"Accept-Version", "Accept-Version", "v2", "v2"},
		{"API-Version", "API-Version", "v3", "v3"},
		{"Accept-Version 优先", "Accept-Version", "v1", "v1"},
		{"无请求头", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotVersion string

			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotVersion = FromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/users", nil)
			if tt.headerName != "" {
				req.Header.Set(tt.headerName, tt.headerValue)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if gotVersion != tt.wantVersion {
				t.Errorf("版本: 期望 %q，得到 %q", tt.wantVersion, gotVersion)
			}
		})
	}
}

func TestHTTPMiddleware_自定义请求头(t *testing.T) {
	middleware := HTTPMiddleware(
		WithPathPrefix(false),
		WithHeader("X-API-Version"),
	)

	var gotVersion string
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("X-API-Version", "v5")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotVersion != "v5" {
		t.Errorf("版本: 期望 %q，得到 %q", "v5", gotVersion)
	}
}

func TestHTTPMiddleware_默认版本(t *testing.T) {
	middleware := HTTPMiddleware(
		WithPathPrefix(false),
		WithDefault("v1"),
	)

	var gotVersion string
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotVersion != "v1" {
		t.Errorf("版本: 期望 %q，得到 %q", "v1", gotVersion)
	}
}

func TestHTTPMiddleware_路径优先于请求头(t *testing.T) {
	middleware := HTTPMiddleware()

	var gotVersion string
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v2/users", nil)
	req.Header.Set("Accept-Version", "v1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotVersion != "v2" {
		t.Errorf("路径版本应优先于请求头，期望 %q，得到 %q", "v2", gotVersion)
	}
}

func TestFromContext_无版本(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ver := FromContext(req.Context())
	if ver != "" {
		t.Errorf("无版本时期望空字符串，得到 %q", ver)
	}
}

func TestFromContext_nil(t *testing.T) {
	//nolint:staticcheck // 测试 nil context 的安全性
	ver := FromContext(nil)
	if ver != "" {
		t.Errorf("nil context 期望空字符串，得到 %q", ver)
	}
}
