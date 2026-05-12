package grpcserver

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/Tsukikage7/servex/v2/testx"
)

// mockRegistrar 测试用 mock registrar.
type mockRegistrar struct {
	registered atomic.Bool
}

func (m *mockRegistrar) RegisterGRPC(server *grpc.Server) {
	m.registered.Store(true)
}

func getAvailablePort(t *testing.T) string {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to get available port: %v", err)
	}
	defer l.Close()
	return l.Addr().String()
}

func TestNew(t *testing.T) {
	t.Run("创建成功", func(t *testing.T) {
		srv := New(
			WithName("test-grpc"),
			WithAddr(":9090"),
			WithLogger(testx.NopLogger()),
		)

		if srv.Name() != "test-grpc" {
			t.Errorf("expected name 'test-grpc', got '%s'", srv.Name())
		}
		if srv.Addr() != ":9090" {
			t.Errorf("expected addr ':9090', got '%s'", srv.Addr())
		}
	})

	t.Run("未设置logger时使用空日志", func(t *testing.T) {
		srv := New(WithAddr(":9090"))
		if srv == nil {
			t.Fatal("server should not be nil")
		}
	})

	t.Run("默认值", func(t *testing.T) {
		srv := New(WithLogger(testx.NopLogger()))

		if srv.Name() != "gRPC" {
			t.Errorf("expected default name 'gRPC', got '%s'", srv.Name())
		}
		if srv.Addr() != ":9090" {
			t.Errorf("expected default addr ':9090', got '%s'", srv.Addr())
		}
	})
}

func TestServer_Register(t *testing.T) {
	srv := New(WithLogger(testx.NopLogger()))

	reg1 := &mockRegistrar{}
	reg2 := &mockRegistrar{}

	result := srv.Register(reg1, reg2)

	// 验证链式调用返回自身
	if result != srv {
		t.Error("Register should return server for chaining")
	}

	if len(srv.opts.services) != 2 {
		t.Errorf("expected 2 services, got %d", len(srv.opts.services))
	}
}

func TestServer_StartAndStop(t *testing.T) {
	addr := getAvailablePort(t)
	srv := New(
		WithAddr(addr),
		WithLogger(testx.NopLogger()),
		WithReflection(true),
	)

	reg := &mockRegistrar{}
	srv.Register(reg)

	ctx, cancel := context.WithCancel(t.Context())

	// 启动服务器
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 验证服务已注册
	if !reg.registered.Load() {
		t.Error("service should be registered")
	}

	// 验证 GRPCServer 不为 nil
	if srv.GRPCServer() == nil {
		t.Error("GRPCServer should not be nil after start")
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
	srv := New(WithLogger(testx.NopLogger()))

	err := srv.Stop(t.Context())
	if err != nil {
		t.Errorf("stopping non-started server should not error: %v", err)
	}
}

func TestServer_GRPCServerBeforeStart(t *testing.T) {
	srv := New(WithLogger(testx.NopLogger()))

	if srv.GRPCServer() != nil {
		t.Error("GRPCServer should be nil before start")
	}
}

func TestServerOptions(t *testing.T) {
	t.Run("WithReflection", func(t *testing.T) {
		srv := New(
			WithLogger(testx.NopLogger()),
			WithReflection(false),
		)
		if srv.opts.enableReflection {
			t.Error("reflection should be disabled")
		}
	})

	t.Run("WithKeepalive", func(t *testing.T) {
		srv := New(
			WithLogger(testx.NopLogger()),
			WithKeepalive(30*time.Second, 10*time.Second),
		)
		if srv.opts.keepaliveTime != 30*time.Second {
			t.Error("keepalive time not set correctly")
		}
		if srv.opts.keepaliveTimeout != 10*time.Second {
			t.Error("keepalive timeout not set correctly")
		}
	})

	t.Run("WithUnaryInterceptor", func(t *testing.T) {
		interceptor := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
		srv := New(
			WithLogger(testx.NopLogger()),
			WithUnaryInterceptor(interceptor),
		)
		if len(srv.opts.unaryInterceptors) != 1 {
			t.Error("unary interceptor not added")
		}
	})

	t.Run("WithStreamInterceptor", func(t *testing.T) {
		interceptor := func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			return handler(srv, ss)
		}
		srv := New(
			WithLogger(testx.NopLogger()),
			WithStreamInterceptor(interceptor),
		)
		if len(srv.opts.streamInterceptors) != 1 {
			t.Error("stream interceptor not added")
		}
	})

	t.Run("WithServerOption", func(t *testing.T) {
		srv := New(
			WithLogger(testx.NopLogger()),
			WithServerOption(grpc.MaxRecvMsgSize(1024)),
		)
		if len(srv.opts.serverOptions) != 1 {
			t.Error("server option not added")
		}
	})
}

func TestServerOptions_Extended(t *testing.T) {
	t.Run("WithHealthTimeout", func(t *testing.T) {
		srv := New(
			WithLogger(testx.NopLogger()),
			WithHealthTimeout(10*time.Second),
		)
		if srv.opts.healthTimeout != 10*time.Second {
			t.Error("health timeout not set correctly")
		}
	})

	t.Run("WithName", func(t *testing.T) {
		srv := New(
			WithLogger(testx.NopLogger()),
			WithHealthTimeout(10*time.Second),
		)
		if srv.opts.healthTimeout != 10*time.Second {
			t.Error("health timeout not set correctly")
		}
	})

	t.Run("WithName", func(t *testing.T) {
		srv := New(
			WithLogger(testx.NopLogger()),
			WithName("custom-grpc"),
		)
		if srv.Name() != "custom-grpc" {
			t.Errorf("expected 'custom-grpc', got '%s'", srv.Name())
		}
	})

	t.Run("WithAddr", func(t *testing.T) {
		srv := New(
			WithLogger(testx.NopLogger()),
			WithAddr(":50051"),
		)
		if srv.Addr() != ":50051" {
			t.Errorf("expected ':50051', got '%s'", srv.Addr())
		}
	})

	t.Run("Health not nil", func(t *testing.T) {
		srv := New(WithLogger(testx.NopLogger()))
		if srv.Health() == nil {
			t.Error("Health should not be nil")
		}
	})

	t.Run("HealthEndpoint", func(t *testing.T) {
		srv := New(WithLogger(testx.NopLogger()))
		ep := srv.HealthEndpoint()
		if ep == nil {
			t.Fatal("HealthEndpoint should not be nil")
		}
		if ep.Addr != ":9090" {
			t.Errorf("expected default addr, got %s", ep.Addr)
		}
	})
}
