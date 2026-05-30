package gateway

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Tsukikage7/servex/v2/testx"
	"github.com/Tsukikage7/servex/v2/transport"
)

func TestNewFromConfig(t *testing.T) {
	reflection := false
	srv, err := NewFromConfig(&transport.GatewayConfig{
		Name:    "orders",
		Version: "1.2.3",
		GRPC: transport.GRPCConfig{
			Addr:             ":19090",
			EnableReflection: &reflection,
			KeepaliveTime:    20 * time.Second,
			KeepaliveTimeout: 5 * time.Second,
		},
		HTTP: transport.HTTPConfig{
			Addr:         ":18080",
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 4 * time.Second,
			IdleTimeout:  30 * time.Second,
		},
		Runtime: transport.RuntimeConfig{
			Recovery: true,
			Response: true,
		},
		Logging: transport.LoggingConfig{
			Enabled:   true,
			SkipPaths: []string{"/healthz"},
		},
		Tracing: transport.TracingConfig{
			Enabled:   true,
			Service:   "orders-api",
			SkipPaths: []string{"/metrics"},
		},
		Metrics: transport.MetricsConfig{
			Enabled:   true,
			Path:      "/metrics",
			Namespace: "orders",
		},
		CORS: transport.CORSConfig{
			Enabled:      true,
			AllowOrigins: []string{"https://example.com"},
		},
		HealthTimeout: 2 * time.Second,
	}, testx.NopLogger())

	assert.NoError(t, err)
	assert.Equal(t, "orders", srv.opts.name)
	assert.Equal(t, "1.2.3", srv.opts.version)
	assert.Equal(t, ":19090", srv.opts.grpcAddr)
	assert.Equal(t, ":18080", srv.opts.httpAddr)
	assert.False(t, srv.opts.enableReflection)
	assert.Equal(t, 20*time.Second, srv.opts.keepaliveTime)
	assert.Equal(t, 5*time.Second, srv.opts.keepaliveTimeout)
	assert.Equal(t, 3*time.Second, srv.opts.httpReadTimeout)
	assert.Equal(t, 4*time.Second, srv.opts.httpWriteTimeout)
	assert.Equal(t, 30*time.Second, srv.opts.httpIdleTimeout)
	assert.Equal(t, 2*time.Second, srv.opts.healthTimeout)
	assert.True(t, srv.opts.enableRecovery)
	assert.True(t, srv.opts.enableResponse)
	assert.True(t, srv.opts.enableLogging)
	assert.Equal(t, []string{"/healthz"}, srv.opts.loggingSkipPaths)
	assert.Equal(t, "orders-api", srv.opts.tracerName)
	assert.Equal(t, []string{"/metrics"}, srv.opts.tracingSkipPaths)
	assert.NotNil(t, srv.opts.metricsCollector)
	assert.True(t, srv.opts.enableCORS)
	assert.Len(t, srv.opts.corsOpts, 2)
}

func TestNewFromConfig_LoggingOnly(t *testing.T) {
	srv, err := NewFromConfig(&transport.GatewayConfig{
		Name: "log-only",
		Logging: transport.LoggingConfig{
			Enabled: true,
		},
	}, testx.NopLogger())

	assert.NoError(t, err)
	assert.True(t, srv.opts.enableLogging)
	assert.Empty(t, srv.opts.tracerName)
	assert.Nil(t, srv.opts.metricsCollector)
	assert.False(t, srv.opts.enableRecovery)
	assert.False(t, srv.opts.enableResponse)
}

func TestNewFromConfig_NilConfig(t *testing.T) {
	srv, err := NewFromConfig(nil, testx.NopLogger())

	assert.Nil(t, srv)
	assert.Error(t, err)
}
