package waf

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPMiddleware_SQL注入(t *testing.T) {
	middleware := HTTPMiddleware(WithRules(SQLInjectionRule()))
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{"正常请求", "name=john", http.StatusOK},
		{"UNION SELECT 注入", "id=1+union+select+*+from+users", http.StatusForbidden},
		{"OR 1=1 注入", "id=1+or+1%3D1", http.StatusForbidden},
		{"DROP TABLE 注入", "id=1%3Bdrop+table+users", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/users?"+tt.query, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("期望状态码 %d，得到 %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestHTTPMiddleware_XSS(t *testing.T) {
	middleware := HTTPMiddleware(WithRules(XSSRule()))
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{"正常请求", "q=hello", http.StatusOK},
		{"Script 标签", "q=%3Cscript%3Ealert(1)%3C/script%3E", http.StatusForbidden},
		{"事件处理器", "q=%3Cimg+onerror%3Dalert(1)%3E", http.StatusForbidden},
		{"JavaScript 协议", "q=javascript:alert(1)", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/search?"+tt.query, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("期望状态码 %d，得到 %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestHTTPMiddleware_路径遍历(t *testing.T) {
	middleware := HTTPMiddleware(WithRules(PathTraversalRule()))
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		url        string
		wantStatus int
	}{
		{"正常请求", "/api/files/readme.txt", http.StatusOK},
		{"路径遍历", "/api/files/../../etc/passwd", http.StatusForbidden},
		{"etc/passwd 参数", "/api/files?path=/etc/passwd", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("期望状态码 %d，得到 %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestHTTPMiddleware_命令注入(t *testing.T) {
	middleware := HTTPMiddleware(WithRules(CommandInjectionRule()))
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{"正常请求", "action=build", http.StatusOK},
		{"管道命令", "action=test%7C%7Cwhoami", http.StatusForbidden},
		{"反引号", "action=%60whoami%60", http.StatusForbidden},
		{"$() 子命令", "action=%24(id)", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/exec?"+tt.query, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("期望状态码 %d，得到 %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestHTTPMiddleware_自定义拦截处理(t *testing.T) {
	var capturedReason string
	middleware := HTTPMiddleware(
		WithRules(SQLInjectionRule()),
		WithBlockHandler(func(w http.ResponseWriter, r *http.Request, reason string) {
			capturedReason = reason
			w.WriteHeader(http.StatusTeapot) // 自定义状态码
		}),
	)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api?id=1+union+select+*+from+users", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Errorf("期望状态码 %d，得到 %d", http.StatusTeapot, rec.Code)
	}
	if capturedReason == "" {
		t.Error("期望拦截原因不为空")
	}
}

func TestHTTPMiddleware_多规则(t *testing.T) {
	middleware := HTTPMiddleware(
		WithRules(SQLInjectionRule(), XSSRule(), PathTraversalRule(), CommandInjectionRule()),
	)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 正常请求应通过
	req := httptest.NewRequest(http.MethodGet, "/api/users?page=1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("正常请求期望 200，得到 %d", rec.Code)
	}
}

func TestHTTPMiddleware_请求头检测(t *testing.T) {
	middleware := HTTPMiddleware(WithRules(XSSRule()))
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/safe", nil)
	req.Header.Set("Referer", "<script>alert(1)</script>")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("期望拦截恶意 Referer 头，得到状态码 %d", rec.Code)
	}
}

func TestHTTPMiddleware_无规则(t *testing.T) {
	middleware := HTTPMiddleware()
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api?id=1+union+select", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("无规则时应放行，期望 200，得到 %d", rec.Code)
	}
}
