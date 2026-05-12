package connectserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	"github.com/Tsukikage7/servex/v2/testx"
	"github.com/Tsukikage7/servex/v2/transport/health"
)

type mockRegistrar struct {
	path      string
	optsCount int
}

func (m *mockRegistrar) RegisterConnect(mux *http.ServeMux, opts ...connect.HandlerOption) {
	m.optsCount = len(opts)
	mux.HandleFunc(m.path, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestServer_Register(t *testing.T) {
	reg := &mockRegistrar{path: "/test.Service/Call"}
	srv := New(
		WithLogger(testx.NopLogger()),
		WithHandlerOptions(connect.WithRequireConnectProtocolHeader()),
	)

	if got := srv.Register(reg); got != srv {
		t.Fatal("Register 应返回 server 以支持链式调用")
	}
	if reg.optsCount != 1 {
		t.Fatalf("handler options 数量 = %d，期望 1", reg.optsCount)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test.Service/Call", nil)
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d，期望 %d", rec.Code, http.StatusAccepted)
	}
}

func TestServer_HealthEndpoint(t *testing.T) {
	srv := New(WithLogger(testx.NopLogger()), WithAddr(":8181"))

	endpoint := srv.HealthEndpoint()
	if endpoint.Type != "http" {
		t.Fatalf("health type = %s，期望 http", endpoint.Type)
	}
	if endpoint.Addr != ":8181" {
		t.Fatalf("health addr = %s，期望 :8181", endpoint.Addr)
	}
	if endpoint.Path != health.DefaultLivenessPath {
		t.Fatalf("health path = %s，期望 %s", endpoint.Path, health.DefaultLivenessPath)
	}
}

func TestServer_HealthHandler(t *testing.T) {
	srv := New(WithLogger(testx.NopLogger()))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, health.DefaultLivenessPath, nil)
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d，期望 %d", rec.Code, http.StatusOK)
	}
}
