// Package debug 提供调试信息面板，支持 JSON API 和 HTML 页面.
package debug

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// Info 调试信息聚合结构.
type Info struct {
	Routes  []Route        `json:"routes,omitzero"`
	Config  map[string]any `json:"config,omitzero"`
	Health  *HealthInfo    `json:"health,omitempty"`
	Metrics map[string]any `json:"metrics,omitzero"`
	Build   BuildInfo      `json:"build"`
}

// Route 已注册的路由信息.
type Route struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Handler string `json:"handler,omitempty"`
}

// HealthInfo 健康检查摘要信息.
type HealthInfo struct {
	Status string         `json:"status"`
	Checks map[string]any `json:"checks,omitzero"`
}

// BuildInfo 构建信息.
type BuildInfo struct {
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	NumCPU    int    `json:"num_cpu"`
	StartTime string `json:"start_time"`
}

// HealthChecker 健康检查回调函数类型.
type HealthChecker func(ctx context.Context) *HealthInfo

// Option Handler 配置选项.
type Option func(*Handler)

// Handler 调试面板 HTTP 处理器.
type Handler struct {
	mu            sync.RWMutex
	routes        []Route
	config        map[string]any
	metrics       map[string]any
	healthChecker HealthChecker
	buildVersion  string
	startTime     time.Time
}

// New 创建调试面板处理器.
func New(opts ...Option) *Handler {
	h := &Handler{
		startTime: time.Now(),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// WithRoutes 设置路由信息.
func WithRoutes(routes []Route) Option {
	return func(h *Handler) {
		h.routes = routes
	}
}

// WithConfig 设置配置信息（敏感字段应在调用方过滤）.
func WithConfig(cfg map[string]any) Option {
	return func(h *Handler) {
		h.config = cfg
	}
}

// WithMetrics 设置指标信息.
func WithMetrics(m map[string]any) Option {
	return func(h *Handler) {
		h.metrics = m
	}
}

// WithHealthChecker 设置健康检查回调.
func WithHealthChecker(checker HealthChecker) Option {
	return func(h *Handler) {
		h.healthChecker = checker
	}
}

// WithBuildVersion 设置构建版本号.
func WithBuildVersion(v string) Option {
	return func(h *Handler) {
		h.buildVersion = v
	}
}

// SetRoutes 动态更新路由信息.
func (h *Handler) SetRoutes(routes []Route) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.routes = routes
}

// SetConfig 动态更新配置信息.
func (h *Handler) SetConfig(cfg map[string]any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.config = cfg
}

// SetMetrics 动态更新指标信息.
func (h *Handler) SetMetrics(m map[string]any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.metrics = m
}

// info 构建调试信息.
func (h *Handler) info(ctx context.Context) Info {
	h.mu.RLock()
	defer h.mu.RUnlock()

	info := Info{
		Routes:  h.routes,
		Config:  h.config,
		Metrics: h.metrics,
		Build: BuildInfo{
			Version:   h.buildVersion,
			GoVersion: runtime.Version(),
			OS:        runtime.GOOS,
			Arch:      runtime.GOARCH,
			NumCPU:    runtime.NumCPU(),
			StartTime: h.startTime.Format(time.RFC3339),
		},
	}

	if h.healthChecker != nil {
		info.Health = h.healthChecker(ctx)
	}

	return info
}

// ServeHTTP 实现 http.Handler 接口.
// GET /debug/info 返回 JSON 格式调试信息.
// GET /debug/     返回 HTML 调试面板.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	switch r.URL.Path {
	case "/debug/info":
		h.serveJSON(w, r)
	case "/debug/", "/debug":
		h.serveHTML(w, r)
	default:
		http.NotFound(w, r)
	}
}

// serveJSON 输出 JSON 格式调试信息.
func (h *Handler) serveJSON(w http.ResponseWriter, r *http.Request) {
	info := h.info(r.Context())
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	json.NewEncoder(w).Encode(info)
}

// serveHTML 输出 HTML 调试面板.
func (h *Handler) serveHTML(w http.ResponseWriter, r *http.Request) {
	info := h.info(r.Context())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	debugTmpl.Execute(w, info)
}

// RegisterRoutes 注册调试路由到 http.ServeMux.
//
// 注意: 调试面板暴露敏感信息（配置、路由、指标等），
// 生产环境中应通过 HTTP 中间件或网络策略限制访问（如仅内网可访问）.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("/debug/info", h)
	mux.Handle("/debug/", h)
	mux.Handle("/debug", h)
}

// debugTmpl HTML 调试面板模板.
var debugTmpl = template.Must(template.New("debug").Parse(`<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="utf-8">
<title>Debug Dashboard</title>
<style>
body{font-family:system-ui,sans-serif;margin:2rem;background:#f8f9fa;color:#212529}
h1{border-bottom:2px solid #dee2e6;padding-bottom:.5rem}
h2{color:#495057;margin-top:1.5rem}
table{border-collapse:collapse;width:100%;margin:.5rem 0}
th,td{padding:.5rem .75rem;border:1px solid #dee2e6;text-align:left}
th{background:#e9ecef;font-weight:600}
tr:hover{background:#f1f3f5}
pre{background:#e9ecef;padding:1rem;border-radius:4px;overflow-x:auto}
.status-up{color:#2f9e44;font-weight:bold}
.status-down{color:#e03131;font-weight:bold}
</style>
</head>
<body>
<h1>Debug Dashboard</h1>

<h2>Build</h2>
<table>
<tr><th>Version</th><td>{{.Build.Version}}</td></tr>
<tr><th>Go</th><td>{{.Build.GoVersion}}</td></tr>
<tr><th>OS / Arch</th><td>{{.Build.OS}} / {{.Build.Arch}}</td></tr>
<tr><th>CPU</th><td>{{.Build.NumCPU}}</td></tr>
<tr><th>Start Time</th><td>{{.Build.StartTime}}</td></tr>
</table>

{{if .Health}}
<h2>Health</h2>
<table>
<tr><th>Status</th><td>{{.Health.Status}}</td></tr>
</table>
{{end}}

{{if .Routes}}
<h2>Routes ({{len .Routes}})</h2>
<table>
<tr><th>Method</th><th>Path</th><th>Handler</th></tr>
{{range .Routes}}
<tr><td>{{.Method}}</td><td>{{.Path}}</td><td>{{.Handler}}</td></tr>
{{end}}
</table>
{{end}}

{{if .Config}}
<h2>Config</h2>
<table>
{{range $k,$v := .Config}}
<tr><th>{{$k}}</th><td>{{$v}}</td></tr>
{{end}}
</table>
{{end}}

{{if .Metrics}}
<h2>Metrics</h2>
<table>
{{range $k,$v := .Metrics}}
<tr><th>{{$k}}</th><td>{{$v}}</td></tr>
{{end}}
</table>
{{end}}

</body>
</html>`))
