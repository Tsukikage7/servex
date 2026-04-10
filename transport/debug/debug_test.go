package debug

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler_JSON(t *testing.T) {
	h := New(
		WithRoutes([]Route{
			{Method: "GET", Path: "/api/users", Handler: "UserHandler"},
			{Method: "POST", Path: "/api/orders", Handler: "OrderHandler"},
		}),
		WithConfig(map[string]any{
			"env":  "development",
			"port": 8080,
		}),
		WithMetrics(map[string]any{
			"requests_total": 12345,
			"goroutines":     42,
		}),
		WithHealthChecker(func(ctx context.Context) *HealthInfo {
			return &HealthInfo{
				Status: "UP",
				Checks: map[string]any{
					"db":    "ok",
					"redis": "ok",
				},
			}
		}),
		WithBuildVersion("1.0.0"),
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/debug/info", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("content-type = %s", ct)
	}

	var info Info
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}

	if len(info.Routes) != 2 {
		t.Errorf("routes count = %d, want 2", len(info.Routes))
	}
	if info.Routes[0].Method != "GET" || info.Routes[0].Path != "/api/users" {
		t.Errorf("route[0] = %+v", info.Routes[0])
	}
	if info.Config["env"] != "development" {
		t.Errorf("config.env = %v", info.Config["env"])
	}
	if info.Health == nil || info.Health.Status != "UP" {
		t.Error("health should be UP")
	}
	if info.Build.Version != "1.0.0" {
		t.Errorf("build.version = %s", info.Build.Version)
	}
	if info.Build.GoVersion == "" {
		t.Error("build.go_version should not be empty")
	}
	if info.Build.StartTime == "" {
		t.Error("build.start_time should not be empty")
	}
}

func TestHandler_HTML(t *testing.T) {
	h := New(
		WithRoutes([]Route{
			{Method: "GET", Path: "/ping"},
		}),
		WithBuildVersion("2.0.0-rc"),
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/debug/", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("content-type = %s", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Debug Dashboard") {
		t.Error("HTML should contain 'Debug Dashboard'")
	}
	if !strings.Contains(body, "2.0.0-rc") {
		t.Error("HTML should contain version")
	}
	if !strings.Contains(body, "/ping") {
		t.Error("HTML should contain route path")
	}
}

func TestHandler_HTMLWithoutSlash(t *testing.T) {
	h := New()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/debug", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	h := New()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/debug/info", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandler_NotFound(t *testing.T) {
	h := New()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/debug/unknown", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandler_NoHealthChecker(t *testing.T) {
	h := New(WithBuildVersion("dev"))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/debug/info", nil)
	h.ServeHTTP(w, r)

	var info Info
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}

	if info.Health != nil {
		t.Error("health should be nil when no checker is set")
	}
}

func TestHandler_DynamicUpdate(t *testing.T) {
	h := New()

	// 初始无路由
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/debug/info", nil)
	h.ServeHTTP(w, r)

	var info Info
	json.Unmarshal(w.Body.Bytes(), &info)
	if len(info.Routes) != 0 {
		t.Fatalf("初始路由数 = %d, want 0", len(info.Routes))
	}

	// 动态设置路由
	h.SetRoutes([]Route{{Method: "GET", Path: "/new"}})
	h.SetConfig(map[string]any{"key": "value"})
	h.SetMetrics(map[string]any{"counter": 1})

	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/debug/info", nil)
	h.ServeHTTP(w, r)

	json.Unmarshal(w.Body.Bytes(), &info)
	if len(info.Routes) != 1 {
		t.Errorf("更新后路由数 = %d, want 1", len(info.Routes))
	}
	if info.Config["key"] != "value" {
		t.Errorf("config.key = %v", info.Config["key"])
	}
}

func TestHandler_RegisterRoutes(t *testing.T) {
	h := New(WithBuildVersion("test"))
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// 测试 /debug/info 路由已注册
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/debug/info", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// 测试 /debug/ 路由已注册
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/debug/", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandler_CacheControl(t *testing.T) {
	h := New()

	// JSON 端点
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/debug/info", nil)
	h.ServeHTTP(w, r)

	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Errorf("JSON Cache-Control = %s", cc)
	}

	// HTML 端点
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/debug/", nil)
	h.ServeHTTP(w, r)

	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Errorf("HTML Cache-Control = %s", cc)
	}
}

func TestHandler_Empty(t *testing.T) {
	h := New()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/debug/info", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var info Info
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}

	// 空 Handler 也应有构建信息
	if info.Build.GoVersion == "" {
		t.Error("build.go_version should not be empty even for empty handler")
	}
	if info.Build.NumCPU == 0 {
		t.Error("build.num_cpu should not be 0")
	}
}
