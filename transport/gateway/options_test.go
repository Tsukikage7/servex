package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/Tsukikage7/servex/v2/auth"
	"github.com/Tsukikage7/servex/v2/httpx/clientip"
	"github.com/Tsukikage7/servex/v2/middleware/cors"
	"github.com/Tsukikage7/servex/v2/middleware/ratelimit"
	"github.com/Tsukikage7/servex/v2/observability/metrics"
	"github.com/Tsukikage7/servex/v2/testx"
)

func TestGateway_Options_Applied(t *testing.T) {
	log := testx.NopLogger()
	limiter := ratelimit.NewTokenBucket(100, 200)
	collector, err := metrics.NewMetrics(&metrics.Config{
		Namespace: "test_gw",
	})
	if err != nil {
		t.Fatalf("创建 metrics collector 失败: %v", err)
	}

	srv := New(
		WithLogger(log),
		WithName("test-gw"),
		WithGRPCAddr(":0"),
		WithHTTPAddr(":0"),
		WithObservability(ObservabilityConfig{
			Logging:          true,
			LoggingSkipPaths: []string{"/health"},
			TracingService:   "test-service",
			Metrics:          collector,
		}),
		WithRuntime(RuntimeConfig{
			Recovery: true,
			Response: true,
		}),
		WithSecurity(SecurityConfig{
			CORS:         true,
			CORSOptions:  []cors.Option{cors.WithAllowOrigins("*")},
			RateLimiter:  limiter,
			ClientIP:     true,
			ClientIPOpts: []clientip.Option{clientip.WithTrustAllProxies()},
		}),
	)

	// 验证所有选项都已正确设置
	if srv.opts.name != "test-gw" {
		t.Errorf("期望 name='test-gw'，实际为 '%s'", srv.opts.name)
	}
	if !srv.opts.enableRecovery {
		t.Error("期望 recovery 已启用")
	}
	if !srv.opts.enableLogging {
		t.Error("期望 logging 已启用")
	}
	if srv.opts.tracerName != "test-service" {
		t.Errorf("期望 tracerName='test-service'，实际为 '%s'", srv.opts.tracerName)
	}
	if srv.opts.metricsCollector == nil {
		t.Error("期望 metrics collector 已设置")
	}
	if !srv.opts.enableCORS {
		t.Error("期望 CORS 已启用")
	}
	if srv.opts.rateLimiter == nil {
		t.Error("期望限流器已设置")
	}
	if !srv.opts.enableClientIP {
		t.Error("期望 clientIP 已启用")
	}
	if !srv.opts.enableResponse {
		t.Error("期望统一响应已启用")
	}

	// 验证 gRPC 拦截器数量：
	// recovery(1) + tracing(1) + logging(1) + metrics(1) + ratelimit(1) + clientip(1) + response(1) = 7 unary
	// recovery(1) + tracing(1) + logging(1) + metrics(1) + ratelimit(1) + clientip(1) = 6 stream
	expectedUnary := 7
	if len(srv.opts.unaryInterceptors) != expectedUnary {
		t.Errorf("期望 %d 个一元拦截器，实际为 %d", expectedUnary, len(srv.opts.unaryInterceptors))
	}
	expectedStream := 6
	if len(srv.opts.streamInterceptors) != expectedStream {
		t.Errorf("期望 %d 个流拦截器，实际为 %d", expectedStream, len(srv.opts.streamInterceptors))
	}
}

func TestGateway_WithObservabilityAndSecurity(t *testing.T) {
	log := testx.NopLogger()
	limiter := ratelimit.NewTokenBucket(100, 200)
	collector, err := metrics.NewMetrics(&metrics.Config{
		Namespace: "test_gateway_grouped_options",
	})
	if err != nil {
		t.Fatalf("创建 metrics collector 失败: %v", err)
	}

	srv := New(
		WithLogger(log),
		WithObservability(ObservabilityConfig{
			TracingService: "gateway",
			TracingSkipPaths: []string{
				"/healthz",
				"/metrics",
			},
			Metrics:          collector,
			Logging:          true,
			LoggingSkipPaths: []string{"/healthz"},
		}),
		WithRuntime(RuntimeConfig{
			Response: true,
			Recovery: true,
		}),
		WithSecurity(SecurityConfig{
			CORS:         true,
			CORSOptions:  []cors.Option{cors.WithAllowOrigins("https://example.com")},
			RateLimiter:  limiter,
			ClientIP:     true,
			ClientIPOpts: []clientip.Option{clientip.WithTrustAllProxies()},
		}),
	)

	assert.Equal(t, "gateway", srv.opts.tracerName)
	assert.Equal(t, []string{"/healthz", "/metrics"}, srv.opts.tracingSkipPaths)
	assert.Same(t, collector, srv.opts.metricsCollector)
	assert.True(t, srv.opts.enableLogging)
	assert.Equal(t, []string{"/healthz"}, srv.opts.loggingSkipPaths)
	assert.True(t, srv.opts.enableResponse)
	assert.True(t, srv.opts.enableRecovery)
	assert.True(t, srv.opts.enableCORS)
	assert.Len(t, srv.opts.corsOpts, 1)
	assert.Same(t, limiter, srv.opts.rateLimiter)
	assert.True(t, srv.opts.enableClientIP)
	assert.Len(t, srv.opts.clientIPOpts, 1)
}

func TestGateway_WithHTTPTLS(t *testing.T) {
	log := testx.NopLogger()

	// 不传入实际 TLS 配置，仅验证选项设置
	srv := New(
		WithLogger(log),
	)

	if srv.opts.httpTLSConfig != nil {
		t.Error("默认不应启用 HTTP TLS")
	}
}

func TestGateway_DefaultOptions(t *testing.T) {
	log := testx.NopLogger()

	srv := New(WithLogger(log))

	if srv.opts.name != "Gateway" {
		t.Errorf("期望默认 name='Gateway'，实际为 '%s'", srv.opts.name)
	}
	if srv.opts.grpcAddr != ":9090" {
		t.Errorf("期望默认 grpcAddr=':9090'，实际为 '%s'", srv.opts.grpcAddr)
	}
	if srv.opts.httpAddr != ":8080" {
		t.Errorf("期望默认 httpAddr=':8080'，实际为 '%s'", srv.opts.httpAddr)
	}
	if srv.opts.httpReadTimeout != 30*time.Second {
		t.Errorf("期望默认 httpReadTimeout=30s，实际为 %v", srv.opts.httpReadTimeout)
	}
}

func TestBuildProtoPolicySkipper_UsesDiscoveredPolicyPublic(t *testing.T) {
	o := defaultOptions()
	o.discoveredPolicies = auth.MethodPolicyMap{
		"/svc.Auth/Login": {
			FullMethod: "/svc.Auth/Login",
			Public:     true,
		},
		"/svc.Order/Create": {
			FullMethod:  "/svc.Order/Create",
			Permissions: []string{"orders:create"},
		},
	}

	skipper := buildProtoPolicySkipper(o)
	if skipper == nil {
		t.Fatal("skipper is nil")
	}
	if !skipper(contextWithMethod("/svc.Auth/Login"), nil) {
		t.Fatal("public discovered policy should skip auth")
	}
	if skipper(contextWithMethod("/svc.Order/Create"), nil) {
		t.Fatal("permission policy should not skip auth")
	}
}

func TestBuildProtoPolicySkipper_NoProtoPolicyDoesNotSkip(t *testing.T) {
	o := defaultOptions()
	o.discoveredPolicies = auth.MethodPolicyMap{}

	skipper := buildProtoPolicySkipper(o)
	if skipper == nil {
		t.Fatal("skipper is nil")
	}
	if skipper(contextWithMethod("/svc.Order/List"), nil) {
		t.Fatal("method without proto public policy should not skip auth")
	}
}

func TestBuiltinAuthPolicies_PublicHealthMethods(t *testing.T) {
	policies := auth.MethodPolicyMap{}
	applyBuiltinAuthPolicies(policies)

	o := defaultOptions()
	o.discoveredPolicies = policies
	skipper := buildProtoPolicySkipper(o)

	if !skipper(contextWithMethod("/grpc.health.v1.Health/Check"), nil) {
		t.Fatal("gRPC health Check should skip auth")
	}
	if !skipper(contextWithMethod("/grpc.health.v1.Health/Watch"), nil) {
		t.Fatal("gRPC health Watch should skip auth")
	}
	if countPublicPolicies(policies) != 2 {
		t.Fatalf("public policy count = %d, want 2", countPublicPolicies(policies))
	}
}

type testServerTransportStream struct {
	method string
}

func (s testServerTransportStream) Method() string {
	return s.method
}

func (testServerTransportStream) SetHeader(metadata.MD) error {
	return nil
}

func (testServerTransportStream) SendHeader(metadata.MD) error {
	return nil
}

func (testServerTransportStream) SetTrailer(metadata.MD) error {
	return nil
}

func contextWithMethod(method string) context.Context {
	return grpc.NewContextWithServerTransportStream(
		context.Background(),
		testServerTransportStream{method: method},
	)
}
