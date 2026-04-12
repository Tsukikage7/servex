package profiling

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNew_NilConfig(t *testing.T) {
	_, err := New(nil)
	if err != ErrNilConfig {
		t.Fatalf("expected ErrNilConfig, got %v", err)
	}
}

func TestNew_InvalidProfileType(t *testing.T) {
	cfg := &Config{
		Types: []ProfileType{"invalid"},
	}
	_, err := New(cfg)
	if err == nil {
		t.Fatal("expected error for invalid profile type")
	}
}

func TestNew_DefaultValues(t *testing.T) {
	cfg := &Config{}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.config.Interval != 60*time.Second {
		t.Errorf("expected default interval 60s, got %v", p.config.Interval)
	}
	if p.config.Duration != 10*time.Second {
		t.Errorf("expected default duration 10s, got %v", p.config.Duration)
	}
}

func TestNew_WithOptions(t *testing.T) {
	cfg := DefaultConfig()
	var logged bool
	logger := func(format string, v ...any) {
		logged = true
		_ = format
		_ = v
	}

	exporter := NewFileExporter(t.TempDir())

	p, err := New(cfg,
		WithLogger(logger),
		WithExporter(exporter),
		WithHTTPPrefix("/custom/pprof"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p.printf("test %s", "msg")
	if !logged {
		t.Error("logger was not called")
	}
	if p.httpPrefix != "/custom/pprof" {
		t.Errorf("expected custom prefix, got %s", p.httpPrefix)
	}
	if p.exporter == nil {
		t.Error("expected exporter to be set")
	}
}

func TestCollect_Heap(t *testing.T) {
	cfg := DefaultConfig()
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prof, err := p.Collect(t.Context(), ProfileHeap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prof.Type != ProfileHeap {
		t.Errorf("expected heap type, got %s", prof.Type)
	}
	if len(prof.Data) == 0 {
		t.Error("expected non-empty profile data")
	}
	if prof.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestCollect_Goroutine(t *testing.T) {
	cfg := DefaultConfig()
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prof, err := p.Collect(t.Context(), ProfileGoroutine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prof.Type != ProfileGoroutine {
		t.Errorf("expected goroutine type, got %s", prof.Type)
	}
	if len(prof.Data) == 0 {
		t.Error("expected non-empty profile data")
	}
}

func TestCollect_Allocs(t *testing.T) {
	cfg := DefaultConfig()
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prof, err := p.Collect(t.Context(), ProfileAllocs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prof.Type != ProfileAllocs {
		t.Errorf("expected allocs type, got %s", prof.Type)
	}
	if len(prof.Data) == 0 {
		t.Error("expected non-empty profile data")
	}
}

func TestCollect_Block(t *testing.T) {
	cfg := DefaultConfig()
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prof, err := p.Collect(t.Context(), ProfileBlock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prof.Type != ProfileBlock {
		t.Errorf("expected block type, got %s", prof.Type)
	}
}

func TestCollect_Mutex(t *testing.T) {
	cfg := DefaultConfig()
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prof, err := p.Collect(t.Context(), ProfileMutex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prof.Type != ProfileMutex {
		t.Errorf("expected mutex type, got %s", prof.Type)
	}
}

func TestCollect_ThreadCreate(t *testing.T) {
	cfg := DefaultConfig()
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prof, err := p.Collect(t.Context(), ProfileThreadCreate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prof.Type != ProfileThreadCreate {
		t.Errorf("expected threadcreate type, got %s", prof.Type)
	}
}

func TestCollect_CPU(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CPU profile test in short mode")
	}

	cfg := &Config{
		Duration: 100 * time.Millisecond,
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prof, err := p.Collect(t.Context(), ProfileCPU)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prof.Type != ProfileCPU {
		t.Errorf("expected cpu type, got %s", prof.Type)
	}
	if len(prof.Data) == 0 {
		t.Error("expected non-empty profile data")
	}
	if prof.Duration != 100*time.Millisecond {
		t.Errorf("expected duration 100ms, got %v", prof.Duration)
	}
}

func TestCollect_InvalidType(t *testing.T) {
	cfg := DefaultConfig()
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Collect(t.Context(), "invalid")
	if err == nil {
		t.Fatal("expected error for invalid profile type")
	}
}

func TestCollect_Labels(t *testing.T) {
	cfg := &Config{
		Labels: map[string]string{
			"service": "test-svc",
			"env":     "testing",
		},
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prof, err := p.Collect(t.Context(), ProfileHeap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prof.Labels["service"] != "test-svc" {
		t.Errorf("expected service label, got %v", prof.Labels)
	}
	if prof.Labels["env"] != "testing" {
		t.Errorf("expected env label, got %v", prof.Labels)
	}
}

func TestStartStop(t *testing.T) {
	cfg := &Config{
		Enabled:  true,
		Types:    []ProfileType{ProfileHeap},
		Interval: 50 * time.Millisecond,
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := t.Context()

	// 启动
	if err := p.Start(ctx); err != nil {
		t.Fatalf("unexpected error on start: %v", err)
	}

	st := p.Status()
	if !st.Running {
		t.Error("expected running=true after start")
	}

	// 重复启动应返回错误
	if err := p.Start(ctx); err != ErrAlreadyRunning {
		t.Fatalf("expected ErrAlreadyRunning, got %v", err)
	}

	// 等待采集
	time.Sleep(150 * time.Millisecond)

	// 停止
	if err := p.Stop(ctx); err != nil {
		t.Fatalf("unexpected error on stop: %v", err)
	}

	st = p.Status()
	if st.Running {
		t.Error("expected running=false after stop")
	}
	if st.CollectedCount == 0 {
		t.Error("expected at least one collection")
	}

	// 重复停止应返回错误
	if err := p.Stop(ctx); err != ErrNotRunning {
		t.Fatalf("expected ErrNotRunning, got %v", err)
	}
}

func TestStart_Disabled(t *testing.T) {
	cfg := &Config{
		Enabled: false,
		Types:   []ProfileType{ProfileHeap},
	}
	var logged string
	p, err := New(cfg, WithLogger(func(format string, v ...any) {
		logged = format
		_ = v
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := p.Start(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logged == "" {
		t.Error("expected log message for disabled profiler")
	}
}

func TestFileExporter(t *testing.T) {
	dir := t.TempDir()
	e := NewFileExporter(dir)

	prof := &Profile{
		Type:      ProfileHeap,
		Data:      []byte("test data"),
		Timestamp: time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		Labels:    map[string]string{"service": "test"},
	}

	if err := e.Export(t.Context(), prof); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(dir, "heap_20260115_103000.pprof")
	data, err := os.ReadFile(expected)
	if err != nil {
		t.Fatalf("failed to read exported file: %v", err)
	}
	if string(data) != "test data" {
		t.Errorf("expected 'test data', got %q", string(data))
	}
}

func TestFileExporter_NestedDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	e := NewFileExporter(dir)

	prof := &Profile{
		Type:      ProfileGoroutine,
		Data:      []byte("goroutine data"),
		Timestamp: time.Now(),
		Labels:    map[string]string{},
	}

	if err := e.Export(t.Context(), prof); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}
}

func TestHandler(t *testing.T) {
	cfg := DefaultConfig()
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := p.Handler()
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	// 测试 pprof index 页面
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_CustomPrefix(t *testing.T) {
	cfg := DefaultConfig()
	p, err := New(cfg, WithHTTPPrefix("/custom/pprof"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := p.Handler()
	req := httptest.NewRequest(http.MethodGet, "/custom/pprof/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestStatus(t *testing.T) {
	cfg := &Config{
		Types: []ProfileType{ProfileHeap, ProfileGoroutine},
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	st := p.Status()
	if st.Running {
		t.Error("expected running=false")
	}
	if st.CollectedCount != 0 {
		t.Errorf("expected 0 collected, got %d", st.CollectedCount)
	}
	if st.ErrorCount != 0 {
		t.Errorf("expected 0 errors, got %d", st.ErrorCount)
	}
	if len(st.ActiveProfiles) != 2 {
		t.Errorf("expected 2 active profiles, got %d", len(st.ActiveProfiles))
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Error("expected enabled=true")
	}
	if cfg.Interval != 60*time.Second {
		t.Errorf("expected 60s interval, got %v", cfg.Interval)
	}
	if cfg.Duration != 10*time.Second {
		t.Errorf("expected 10s duration, got %v", cfg.Duration)
	}
	if len(cfg.Types) != 3 {
		t.Errorf("expected 3 default types, got %d", len(cfg.Types))
	}
}

func TestStartWithExporter(t *testing.T) {
	dir := t.TempDir()
	exporter := NewFileExporter(dir)

	cfg := &Config{
		Enabled:  true,
		Types:    []ProfileType{ProfileHeap},
		Interval: 50 * time.Millisecond,
	}
	p, err := New(cfg, WithExporter(exporter))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := t.Context()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("unexpected error on start: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	if err := p.Stop(ctx); err != nil {
		t.Fatalf("unexpected error on stop: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected exported profile files")
	}
}

func TestWithLogger_Nil(t *testing.T) {
	cfg := DefaultConfig()
	p, err := New(cfg, WithLogger(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 不应 panic
	p.printf("test %s", "msg")
}

func TestWithExporter_Nil(t *testing.T) {
	cfg := DefaultConfig()
	p, err := New(cfg, WithExporter(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.exporter != nil {
		t.Error("expected nil exporter")
	}
}
