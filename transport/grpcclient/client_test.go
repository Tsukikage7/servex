package grpcclient

import (
	"context"
	"crypto/tls"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/Tsukikage7/servex/v2/middleware/circuitbreaker"
	"github.com/Tsukikage7/servex/v2/testx"
	"github.com/Tsukikage7/servex/v2/transport"
)

// mockDiscovery 测试用 mock discovery.
type mockDiscovery struct {
	addrs []string
	err   error
}

func (m *mockDiscovery) Register(ctx context.Context, serviceName, address string) (string, error) {
	return "", nil
}
func (m *mockDiscovery) RegisterWithProtocol(ctx context.Context, serviceName, address, protocol string) (string, error) {
	return "", nil
}
func (m *mockDiscovery) RegisterWithHealthEndpoint(ctx context.Context, serviceName, address, protocol string, healthEndpoint *transport.HealthEndpoint) (string, error) {
	return "", nil
}
func (m *mockDiscovery) Unregister(ctx context.Context, serviceID string) error { return nil }
func (m *mockDiscovery) Discover(ctx context.Context, serviceName string) ([]string, error) {
	return m.addrs, m.err
}
func (m *mockDiscovery) Close() error { return nil }

func TestNew(t *testing.T) {
	t.Run("创建成功", func(t *testing.T) {
		disc := &mockDiscovery{addrs: []string{"localhost:9090"}}
		client, err := New(
			WithName("test-client"),
			WithServiceName("test-service"),
			WithDiscovery(disc),
			WithLogger(testx.NopLogger()),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer client.Close()

		if client.Conn() == nil {
			t.Error("connection should not be nil")
		}
	})

	t.Run("未设置serviceName时返回错误", func(t *testing.T) {
		_, err := New(
			WithDiscovery(&mockDiscovery{addrs: []string{"localhost:9090"}}),
			WithLogger(testx.NopLogger()),
		)
		if err == nil {
			t.Error("expected error when serviceName not set")
		}
	})

	t.Run("未设置discovery时返回错误", func(t *testing.T) {
		_, err := New(
			WithServiceName("test-service"),
			WithLogger(testx.NopLogger()),
		)
		if err == nil {
			t.Error("expected error when discovery not set")
		}
	})

	t.Run("未设置logger时使用空日志", func(t *testing.T) {
		client, err := New(
			WithServiceName("test-service"),
			WithDiscovery(&mockDiscovery{addrs: []string{"localhost:9090"}}),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client == nil {
			t.Fatal("client should not be nil")
		}
	})

	t.Run("服务发现失败", func(t *testing.T) {
		disc := &mockDiscovery{err: errors.New("discovery error")}
		_, err := New(
			WithServiceName("test-service"),
			WithDiscovery(disc),
			WithLogger(testx.NopLogger()),
		)
		if err == nil {
			t.Error("expected error when discovery fails")
		}
		if !errors.Is(err, ErrDiscoveryFailed) {
			t.Errorf("expected ErrDiscoveryFailed, got %v", err)
		}
	})

	t.Run("未找到服务实例", func(t *testing.T) {
		disc := &mockDiscovery{addrs: []string{}}
		_, err := New(
			WithServiceName("test-service"),
			WithDiscovery(disc),
			WithLogger(testx.NopLogger()),
		)
		if err == nil {
			t.Error("expected error when no service found")
		}
		if !errors.Is(err, ErrServiceNotFound) {
			t.Errorf("expected ErrServiceNotFound, got %v", err)
		}
	})
}

func TestClient_Close(t *testing.T) {
	t.Run("关闭连接", func(t *testing.T) {
		disc := &mockDiscovery{addrs: []string{"localhost:9090"}}
		client, err := New(
			WithServiceName("test-service"),
			WithDiscovery(disc),
			WithLogger(testx.NopLogger()),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = client.Close()
		if err != nil {
			t.Errorf("unexpected close error: %v", err)
		}
	})

	t.Run("关闭nil连接", func(t *testing.T) {
		client := &Client{opts: &options{logger: testx.NopLogger(), name: "test", serviceName: "test"}}
		err := client.Close()
		if err != nil {
			t.Errorf("closing nil connection should not error: %v", err)
		}
	})
}

func TestClientOptions(t *testing.T) {
	t.Run("WithName", func(t *testing.T) {
		opts := defaultOptions()
		WithName("custom-name")(opts)
		if opts.name != "custom-name" {
			t.Errorf("expected name 'custom-name', got '%s'", opts.name)
		}
	})

	t.Run("WithInterceptors", func(t *testing.T) {
		opts := defaultOptions()
		interceptor := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, callOpts ...grpc.CallOption) error {
			return invoker(ctx, method, req, reply, cc, callOpts...)
		}
		WithInterceptors(interceptor)(opts)
		if len(opts.interceptors) != 1 {
			t.Error("interceptor not added")
		}
	})

	t.Run("WithDialOptions", func(t *testing.T) {
		opts := defaultOptions()
		WithDialOptions(grpc.WithAuthority("test"))(opts)
		if len(opts.dialOptions) != 1 {
			t.Error("dial option not added")
		}
	})

	t.Run("默认值", func(t *testing.T) {
		opts := defaultOptions()
		if opts.name != "gRPC-Client" {
			t.Errorf("expected default name 'gRPC-Client', got '%s'", opts.name)
		}
	})
}

func TestWithTLS_Option(t *testing.T) {
	opts := defaultOptions()
	tlsCfg := &tls.Config{InsecureSkipVerify: true}
	WithTLS(tlsCfg)(opts)
	if opts.tlsConfig == nil {
		t.Fatal("TLS config should not be nil")
	}
	if !opts.tlsConfig.InsecureSkipVerify {
		t.Error("TLS config should have InsecureSkipVerify=true")
	}
}

func TestWithRetry_Option(t *testing.T) {
	opts := defaultOptions()
	WithRetry(3, 100*time.Millisecond)(opts)
	if opts.retryMaxAttempts != 3 {
		t.Errorf("expected retryMaxAttempts=3, got %d", opts.retryMaxAttempts)
	}
	if opts.retryBackoff != 100*time.Millisecond {
		t.Errorf("expected retryBackoff=100ms, got %v", opts.retryBackoff)
	}
}

func TestWithBalancer_Option(t *testing.T) {
	opts := defaultOptions()
	WithBalancer("round_robin")(opts)
	if opts.balancerPolicy != "round_robin" {
		t.Errorf("expected balancerPolicy='round_robin', got '%s'", opts.balancerPolicy)
	}
}

func TestWithCircuitBreaker_Option(t *testing.T) {
	opts := defaultOptions()
	cb := circuitbreaker.New()
	WithCircuitBreaker(cb)(opts)
	if opts.circuitBreaker == nil {
		t.Fatal("circuit breaker should not be nil")
	}
}

func TestWithTracing_Option(t *testing.T) {
	opts := defaultOptions()
	WithTracing("test-service")(opts)
	if opts.tracerName != "test-service" {
		t.Errorf("expected tracerName='test-service', got '%s'", opts.tracerName)
	}
}

func TestWithLogging_Option(t *testing.T) {
	opts := defaultOptions()
	WithLogging()(opts)
	if !opts.enableLogging {
		t.Error("logging should be enabled")
	}
}

func TestWithTimeout_Option(t *testing.T) {
	opts := defaultOptions()
	WithTimeout(5 * time.Second)(opts)
	if opts.timeout != 5*time.Second {
		t.Errorf("expected timeout=5s, got %v", opts.timeout)
	}
}

func TestWithKeepalive_Option(t *testing.T) {
	opts := defaultOptions()
	WithKeepalive(30*time.Second, 10*time.Second)(opts)
	if opts.keepaliveTime != 30*time.Second {
		t.Errorf("expected keepaliveTime=30s, got %v", opts.keepaliveTime)
	}
	if opts.keepaliveTimeout != 10*time.Second {
		t.Errorf("expected keepaliveTimeout=10s, got %v", opts.keepaliveTimeout)
	}
}

func TestWithStreamInterceptors_Option(t *testing.T) {
	opts := defaultOptions()
	si := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, callOpts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(ctx, desc, cc, method, callOpts...)
	}
	WithStreamInterceptors(si)(opts)
	if len(opts.streamInterceptors) != 1 {
		t.Error("stream interceptor not added")
	}
}

func TestNewFromConfig(t *testing.T) {
	t.Run("基本配置", func(t *testing.T) {
		client, err := NewFromConfig(&Config{
			Addr:        "localhost:9090",
			ServiceName: "test-service",
			Timeout:     5 * time.Second,
		}, WithLogger(testx.NopLogger()))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer client.Close()

		if client.Conn() == nil {
			t.Error("connection should not be nil")
		}
	})

	t.Run("缺少地址", func(t *testing.T) {
		_, err := NewFromConfig(&Config{})
		if err == nil {
			t.Error("expected error when addr is missing")
		}
	})

	t.Run("带重试配置", func(t *testing.T) {
		client, err := NewFromConfig(&Config{
			Addr: "localhost:9090",
			Retry: &RetryConfig{
				MaxAttempts: 3,
				Backoff:     100 * time.Millisecond,
			},
		}, WithLogger(testx.NopLogger()))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer client.Close()
	})

	t.Run("带负载均衡配置", func(t *testing.T) {
		client, err := NewFromConfig(&Config{
			Addr:     "localhost:9090",
			Balancer: "round_robin",
		}, WithLogger(testx.NopLogger()))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer client.Close()
	})

	t.Run("带Keepalive配置", func(t *testing.T) {
		client, err := NewFromConfig(&Config{
			Addr: "localhost:9090",
			Keepalive: &KeepaliveConfig{
				Time:    30 * time.Second,
				Timeout: 10 * time.Second,
			},
		}, WithLogger(testx.NopLogger()))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer client.Close()
	})

	t.Run("启用Tracing", func(t *testing.T) {
		client, err := NewFromConfig(&Config{
			Addr:          "localhost:9090",
			ServiceName:   "test-service",
			EnableTracing: true,
		}, WithLogger(testx.NopLogger()))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer client.Close()
	})
}

func TestConfigDefaults(t *testing.T) {
	cfg := &Config{}
	if cfg.Balancer != "" {
		t.Errorf("default balancer should be empty, got '%s'", cfg.Balancer)
	}
	if cfg.Timeout != 0 {
		t.Errorf("default timeout should be 0, got %v", cfg.Timeout)
	}
	if cfg.EnableTracing {
		t.Error("default enable_tracing should be false")
	}
	if cfg.EnableMetrics {
		t.Error("default enable_metrics should be false")
	}
	if cfg.TLS != nil {
		t.Error("default TLS should be nil")
	}
}

func TestNewWithTLS(t *testing.T) {
	disc := &mockDiscovery{addrs: []string{"localhost:9090"}}
	tlsCfg := &tls.Config{InsecureSkipVerify: true}
	client, err := New(
		WithServiceName("test-service"),
		WithDiscovery(disc),
		WithLogger(testx.NopLogger()),
		WithTLS(tlsCfg),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer client.Close()

	if client.Conn() == nil {
		t.Error("connection should not be nil")
	}
}

func TestNewWithAllOptions(t *testing.T) {
	disc := &mockDiscovery{addrs: []string{"localhost:9090"}}
	cb := circuitbreaker.New()
	client, err := New(
		WithServiceName("test-service"),
		WithDiscovery(disc),
		WithLogger(testx.NopLogger()),
		WithRetry(3, 100*time.Millisecond),
		WithCircuitBreaker(cb),
		WithTracing("test-service"),
		WithLogging(),
		WithBalancer("round_robin"),
		WithTimeout(5*time.Second),
		WithKeepalive(30*time.Second, 10*time.Second),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer client.Close()
}
