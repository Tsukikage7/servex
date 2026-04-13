package testx

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

func TestNopLogger(t *testing.T) {
	l := NopLogger()
	// 确保所有方法都能正常调用不会 panic.
	l.Debug("debug")
	l.Debugf("debug %s", "msg")
	l.Info("info")
	l.Infof("info %s", "msg")
	l.Warn("warn")
	l.Warnf("warn %s", "msg")
	l.Error("error")
	l.Errorf("error %s", "msg")

	// With 应返回同类型 logger.
	l2 := l.With(logger.Field{Key: "k", Value: "v"})
	if l2 == nil {
		t.Fatal("With 返回了 nil")
	}
	l2.Info("with field")

	// Sync 和 Close 应返回 nil.
	if err := l.Sync(); err != nil {
		t.Fatalf("Sync 返回了非 nil 错误: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close 返回了非 nil 错误: %v", err)
	}
}

func TestTestLogger(t *testing.T) {
	l := TestLogger(t)
	// 验证所有方法可以正常执行，输出会通过 t.Log 记录.
	l.Debug("debug message")
	l.Debugf("debug %s", "formatted")
	l.Info("info message")
	l.Infof("info %d", 42)
	l.Warn("warn message")
	l.Error("error message")

	l2 := l.With(logger.Field{Key: "request_id", Value: "abc123"})
	l2.Info("with field message")
}

func TestLoadJSON(t *testing.T) {
	// 创建临时 JSON 文件.
	type sample struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	data, _ := json.Marshal(sample{Name: "hello", Value: 42})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("写入临时文件失败: %v", err)
	}

	result := LoadJSON[sample](t, path)
	if result.Name != "hello" {
		t.Errorf("Name 不匹配: 期望 %q, 实际 %q", "hello", result.Name)
	}
	if result.Value != 42 {
		t.Errorf("Value 不匹配: 期望 %d, 实际 %d", 42, result.Value)
	}
}

func TestGolden(t *testing.T) {
	// 使用临时目录替代 testdata 目录进行测试.
	dir := t.TempDir()
	goldenDir := filepath.Join(dir, "testdata")
	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		t.Fatal(err)
	}

	goldenFile := filepath.Join(goldenDir, "sample.golden")
	expected := []byte("hello golden\n")
	if err := os.WriteFile(goldenFile, expected, 0o644); err != nil {
		t.Fatal(err)
	}

	// 保存并切换工作目录到临时目录.
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	// 匹配场景.
	Golden(t, "sample", expected)
}

func TestLoadYAML(t *testing.T) {
	type sample struct {
		Name  string `yaml:"name"`
		Value int    `yaml:"value"`
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	content := "name: hello\nvalue: 42\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入临时文件失败: %v", err)
	}

	result := LoadYAML[sample](t, path)
	if result.Name != "hello" {
		t.Errorf("Name 不匹配: 期望 %q, 实际 %q", "hello", result.Name)
	}
	if result.Value != 42 {
		t.Errorf("Value 不匹配: 期望 %d, 实际 %d", 42, result.Value)
	}
}

func TestHTTPTestServer(t *testing.T) {
	handler := http.NewServeMux()
	handler.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"msg":"hello"}`))
	})
	handler.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		json.NewEncoder(w).Encode(body)
	})

	srv := NewHTTPTestServer(handler)
	defer srv.Close()

	t.Run("Get", func(t *testing.T) {
		resp := srv.Get("/hello")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("PostJSON", func(t *testing.T) {
		resp := srv.PostJSON("/echo", map[string]string{"key": "value"})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("with middleware", func(t *testing.T) {
		var mwCalled bool
		mw := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mwCalled = true
				next.ServeHTTP(w, r)
			})
		}

		innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		mwSrv := NewHTTPTestServer(innerHandler, mw)
		defer mwSrv.Close()

		resp := mwSrv.Get("/anything")
		resp.Body.Close()
		if !mwCalled {
			t.Error("middleware should be called")
		}
	})
}

func TestGoldenJSON(t *testing.T) {
	dir := t.TempDir()
	goldenDir := filepath.Join(dir, "testdata")
	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create expected golden content
	type sample struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	data, _ := json.MarshalIndent(sample{Name: "test", Value: 99}, "", "  ")
	data = append(data, '\n')
	goldenFile := filepath.Join(goldenDir, "json_test.golden")
	if err := os.WriteFile(goldenFile, data, 0o644); err != nil {
		t.Fatal(err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	GoldenJSON(t, "json_test", sample{Name: "test", Value: 99})
}
