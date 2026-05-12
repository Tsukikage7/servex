package httpserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/testx"
)

func getAvailablePort(t *testing.T) string {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to get available port: %v", err)
	}
	defer l.Close()
	return l.Addr().String()
}

func TestNew(t *testing.T) {
	mux := http.NewServeMux()

	t.Run("创建成功", func(t *testing.T) {
		srv := New(mux,
			WithName("test-http"),
			WithAddr(":8080"),
			WithLogger(testx.NopLogger()),
		)

		if srv.Name() != "test-http" {
			t.Errorf("expected name 'test-http', got '%s'", srv.Name())
		}
		if srv.Addr() != ":8080" {
			t.Errorf("expected addr ':8080', got '%s'", srv.Addr())
		}
		// Handler() 返回包装后的 handler（包含健康检查中间件）
		if srv.Handler() == nil {
			t.Error("handler should not be nil")
		}
		// 验证健康检查管理器已创建
		if srv.Health() == nil {
			t.Error("health manager should not be nil")
		}
	})

	t.Run("未设置logger时使用空日志", func(t *testing.T) {
		srv := New(mux, WithAddr(":8080"))
		if srv == nil {
			t.Fatal("server should not be nil")
		}
	})

	t.Run("默认值", func(t *testing.T) {
		srv := New(mux, WithLogger(testx.NopLogger()))

		if srv.Name() != "HTTP" {
			t.Errorf("expected default name 'HTTP', got '%s'", srv.Name())
		}
		if srv.Addr() != ":8080" {
			t.Errorf("expected default addr ':8080', got '%s'", srv.Addr())
		}
	})
}

func TestServer_StartAndStop(t *testing.T) {
	addr := getAvailablePort(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := New(mux,
		WithAddr(addr),
		WithLogger(testx.NopLogger()),
	)

	ctx, cancel := context.WithCancel(t.Context())

	// 启动服务器
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 验证服务器可访问
	resp, err := http.Get("http://" + addr + "/health")
	if err != nil {
		t.Fatalf("failed to reach server: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// 停止服务器
	cancel()
	stopCtx, stopCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer stopCancel()

	if err := srv.Stop(stopCtx); err != nil {
		t.Errorf("unexpected stop error: %v", err)
	}

	// 等待 Start 返回
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("unexpected start error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("timeout waiting for server to stop")
	}
}

func TestServer_StopNotStarted(t *testing.T) {
	mux := http.NewServeMux()
	srv := New(mux, WithLogger(testx.NopLogger()))

	err := srv.Stop(t.Context())
	if err != nil {
		t.Errorf("stopping non-started server should not error: %v", err)
	}
}

func TestServerOptions(t *testing.T) {
	mux := http.NewServeMux()

	t.Run("ReadTimeout", func(t *testing.T) {
		srv := New(mux,
			WithLogger(testx.NopLogger()),
			WithTimeout(10*time.Second, 0, 0),
		)
		if srv.opts.readTimeout != 10*time.Second {
			t.Error("read timeout not set correctly")
		}
	})

	t.Run("WriteTimeout", func(t *testing.T) {
		srv := New(mux,
			WithLogger(testx.NopLogger()),
			WithTimeout(0, 15*time.Second, 0),
		)
		if srv.opts.writeTimeout != 15*time.Second {
			t.Error("write timeout not set correctly")
		}
	})

	t.Run("IdleTimeout", func(t *testing.T) {
		srv := New(mux,
			WithLogger(testx.NopLogger()),
			WithTimeout(0, 0, 60*time.Second),
		)
		if srv.opts.idleTimeout != 60*time.Second {
			t.Error("idle timeout not set correctly")
		}
	})

	t.Run("默认超时值", func(t *testing.T) {
		srv := New(mux, WithLogger(testx.NopLogger()))

		if srv.opts.readTimeout != 30*time.Second {
			t.Errorf("expected default read timeout 30s, got %v", srv.opts.readTimeout)
		}
		if srv.opts.writeTimeout != 30*time.Second {
			t.Errorf("expected default write timeout 30s, got %v", srv.opts.writeTimeout)
		}
		if srv.opts.idleTimeout != 120*time.Second {
			t.Errorf("expected default idle timeout 120s, got %v", srv.opts.idleTimeout)
		}
	})
}

func TestEndpointHandler(t *testing.T) {
	t.Run("basic endpoint", func(t *testing.T) {
		ep := func(ctx context.Context, req any) (any, error) {
			return map[string]string{"msg": "hello"}, nil
		}
		dec := func(ctx context.Context, r *http.Request) (any, error) {
			return nil, nil
		}

		handler := NewEndpointHandler(ep, dec, EncodeJSONResponse)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if rec.Header().Get("Content-Type") != "application/json; charset=utf-8" {
			t.Errorf("unexpected content-type: %s", rec.Header().Get("Content-Type"))
		}
	})

	t.Run("decode error", func(t *testing.T) {
		ep := func(ctx context.Context, req any) (any, error) {
			return nil, nil
		}
		dec := func(ctx context.Context, r *http.Request) (any, error) {
			return nil, errors.New("decode error")
		}

		handler := NewEndpointHandler(ep, dec, EncodeJSONResponse)
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("endpoint error", func(t *testing.T) {
		ep := func(ctx context.Context, req any) (any, error) {
			return nil, errors.New("endpoint error")
		}
		dec := func(ctx context.Context, r *http.Request) (any, error) {
			return nil, nil
		}

		handler := NewEndpointHandler(ep, dec, EncodeJSONResponse)
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("with before func", func(t *testing.T) {
		var beforeCalled bool
		ep := func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		}
		dec := func(ctx context.Context, r *http.Request) (any, error) {
			return nil, nil
		}

		handler := NewEndpointHandler(ep, dec, EncodeJSONResponse,
			WithBefore(func(ctx context.Context, r *http.Request) context.Context {
				beforeCalled = true
				return ctx
			}),
		)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if !beforeCalled {
			t.Error("before func should be called")
		}
	})

	t.Run("with after func", func(t *testing.T) {
		var afterCalled bool
		ep := func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		}
		dec := func(ctx context.Context, r *http.Request) (any, error) {
			return nil, nil
		}

		handler := NewEndpointHandler(ep, dec, EncodeJSONResponse,
			WithAfter(func(ctx context.Context, w http.ResponseWriter) context.Context {
				afterCalled = true
				return ctx
			}),
		)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if !afterCalled {
			t.Error("after func should be called")
		}
	})
}

// mockHTTPRegistrar 测试用 mock registrar.
type mockHTTPRegistrar struct {
	registered bool
	path       string
}

func (m *mockHTTPRegistrar) RegisterHTTP(router *Router) {
	m.registered = true
	router.GET(m.path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("mock"))
	}))
}

func TestServer_Register(t *testing.T) {
	t.Run("使用Router时注册服务", func(t *testing.T) {
		router := NewRouter()
		srv := New(router, WithLogger(testx.NopLogger()))

		reg1 := &mockHTTPRegistrar{path: "/mock1"}
		reg2 := &mockHTTPRegistrar{path: "/mock2"}

		result := srv.Register(reg1, reg2)

		// 验证链式调用返回自身
		if result != srv {
			t.Error("Register should return server for chaining")
		}

		// 验证 registrar 被调用
		if !reg1.registered {
			t.Error("registrar 1 should be registered")
		}
		if !reg2.registered {
			t.Error("registrar 2 should be registered")
		}

		// 验证注册的路由可访问
		req := httptest.NewRequest(http.MethodGet, "/mock1", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Body.String() != "mock" {
			t.Errorf("expected 'mock', got %q", rec.Body.String())
		}
	})

	t.Run("使用非Router handler时自动创建Router", func(t *testing.T) {
		mux := http.NewServeMux()
		srv := New(mux, WithLogger(testx.NopLogger()))

		reg := &mockHTTPRegistrar{path: "/test-reg"}
		result := srv.Register(reg)

		if result != srv {
			t.Error("Register should return server for chaining")
		}
		if !reg.registered {
			t.Error("registrar should be registered")
		}
		// router 应该已被创建
		if srv.router == nil {
			t.Error("router should be created when handler is not a Router")
		}
	})

	t.Run("多次调用Register", func(t *testing.T) {
		router := NewRouter()
		srv := New(router, WithLogger(testx.NopLogger()))

		reg1 := &mockHTTPRegistrar{path: "/multi1"}
		reg2 := &mockHTTPRegistrar{path: "/multi2"}

		srv.Register(reg1).Register(reg2)

		if !reg1.registered {
			t.Error("registrar 1 should be registered")
		}
		if !reg2.registered {
			t.Error("registrar 2 should be registered")
		}
	})

	t.Run("空registrars不影响服务器", func(t *testing.T) {
		router := NewRouter()
		srv := New(router, WithLogger(testx.NopLogger()))

		result := srv.Register()
		if result != srv {
			t.Error("Register with no args should return server")
		}
	})
}

func TestRouter(t *testing.T) {
	t.Run("basic routing", func(t *testing.T) {
		router := NewRouter()
		router.GET("/hello", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("hello"))
		}))

		req := httptest.NewRequest(http.MethodGet, "/hello", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Body.String() != "hello" {
			t.Errorf("expected 'hello', got %q", rec.Body.String())
		}
	})

	t.Run("group prefix", func(t *testing.T) {
		router := NewRouter()
		api := router.Group("/api/v1")
		api.GET("/users", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("users"))
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Body.String() != "users" {
			t.Errorf("expected 'users', got %q", rec.Body.String())
		}
	})

	t.Run("middleware execution order", func(t *testing.T) {
		var order []string
		mw := func(name string) Middleware {
			return func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					order = append(order, name)
					next.ServeHTTP(w, r)
				})
			}
		}

		router := NewRouter(mw("global"))
		router.GET("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "handler")
		}), mw("route"))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		expected := []string{"global", "route", "handler"}
		if len(order) != len(expected) {
			t.Fatalf("expected %v, got %v", expected, order)
		}
		for i, v := range expected {
			if order[i] != v {
				t.Errorf("order[%d]: expected %q, got %q", i, v, order[i])
			}
		}
	})

	t.Run("Use adds middleware", func(t *testing.T) {
		var useCalled bool
		router := NewRouter()
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				useCalled = true
				next.ServeHTTP(w, r)
			})
		})
		router.GET("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(httptest.NewRecorder(), req)

		if !useCalled {
			t.Error("Use middleware should be called")
		}
	})

	t.Run("all HTTP methods", func(t *testing.T) {
		router := NewRouter()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
		router.POST("/res", handler)
		router.PUT("/res", handler)
		router.PATCH("/res", handler)
		router.DELETE("/res", handler)
		router.Handle("OPTIONS /res", handler)

		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions} {
			req := httptest.NewRequest(method, "/res", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("%s /res: expected 200, got %d", method, rec.Code)
			}
		}
	})
}
