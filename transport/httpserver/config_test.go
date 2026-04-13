package httpserver

import (
	"net/http"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/testx"
)

func TestNewFromConfig(t *testing.T) {
	mux := http.NewServeMux()
	log := testx.NopLogger()

	t.Run("基本配置", func(t *testing.T) {
		cfg := &Config{
			Name: "test-cfg",
			Addr: ":9090",
		}
		srv, err := NewFromConfig(mux, cfg, log)
		if err != nil {
			t.Fatalf("NewFromConfig 失败: %v", err)
		}

		if srv.Name() != "test-cfg" {
			t.Errorf("期望 name='test-cfg'，实际为 '%s'", srv.Name())
		}
		if srv.Addr() != ":9090" {
			t.Errorf("期望 addr=':9090'，实际为 '%s'", srv.Addr())
		}
	})

	t.Run("超时配置", func(t *testing.T) {
		cfg := &Config{
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		}
		srv, err := NewFromConfig(mux, cfg, log)
		if err != nil {
			t.Fatalf("NewFromConfig 失败: %v", err)
		}

		if srv.opts.readTimeout != 10*time.Second {
			t.Errorf("期望 readTimeout=10s，实际为 %v", srv.opts.readTimeout)
		}
		if srv.opts.writeTimeout != 15*time.Second {
			t.Errorf("期望 writeTimeout=15s，实际为 %v", srv.opts.writeTimeout)
		}
		if srv.opts.idleTimeout != 60*time.Second {
			t.Errorf("期望 idleTimeout=60s，实际为 %v", srv.opts.idleTimeout)
		}
	})

	t.Run("Profiling 配置", func(t *testing.T) {
		cfg := &Config{Profiling: "/debug/pprof"}
		srv, err := NewFromConfig(mux, cfg, log)
		if err != nil {
			t.Fatalf("NewFromConfig 失败: %v", err)
		}

		if srv.opts.profiling != "/debug/pprof" {
			t.Errorf("期望 profiling='/debug/pprof'，实际为 '%s'", srv.opts.profiling)
		}
	})

	t.Run("空配置使用默认值", func(t *testing.T) {
		cfg := &Config{}
		srv, err := NewFromConfig(mux, cfg, log)
		if err != nil {
			t.Fatalf("NewFromConfig 失败: %v", err)
		}

		if srv.Name() != "HTTP" {
			t.Errorf("期望默认 name='HTTP'，实际为 '%s'", srv.Name())
		}
		if srv.Addr() != ":8080" {
			t.Errorf("期望默认 addr=':8080'，实际为 '%s'", srv.Addr())
		}
	})

	t.Run("附加选项覆盖配置", func(t *testing.T) {
		cfg := &Config{Name: "from-config"}
		srv, err := NewFromConfig(mux, cfg, log, WithName("from-option"))
		if err != nil {
			t.Fatalf("NewFromConfig 失败: %v", err)
		}

		if srv.Name() != "from-option" {
			t.Errorf("附加选项应覆盖配置，期望 'from-option'，实际为 '%s'", srv.Name())
		}
	})

	t.Run("完整配置", func(t *testing.T) {
		cfg := &Config{
			Name:         "full",
			Addr:         ":3000",
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  30 * time.Second,
			Profiling:    "/debug/pprof",
		}
		srv, err := NewFromConfig(mux, cfg, log)
		if err != nil {
			t.Fatalf("NewFromConfig 失败: %v", err)
		}

		if srv.Name() != "full" {
			t.Errorf("期望 name='full'，实际为 '%s'", srv.Name())
		}
		if srv.opts.readTimeout != 5*time.Second {
			t.Errorf("期望 readTimeout=5s，实际为 %v", srv.opts.readTimeout)
		}
		if srv.opts.profiling != "/debug/pprof" {
			t.Errorf("期望 profiling='/debug/pprof'，实际为 '%s'", srv.opts.profiling)
		}
	})
}
