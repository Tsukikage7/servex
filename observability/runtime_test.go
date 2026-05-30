package observability

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/metric/noop"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

func TestNewRuntimeSupportsLogOnlyService(t *testing.T) {
	cfg := Config{
		Service: ServiceConfig{
			Name:    "billing",
			Version: "1.2.3",
		},
		Logger: logger.Config{
			Level:  logger.LevelInfo,
			Format: logger.FormatConsole,
			Output: logger.OutputConsole,
		},
		Tracing: TracingConfig{Enabled: false},
		Metrics: MetricsConfig{Enabled: false},
	}

	rt, err := NewRuntime(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	defer rt.Shutdown(context.Background())

	if rt.Logger() == nil {
		t.Fatal("Logger() returned nil")
	}
	if rt.TracerProvider() == nil {
		t.Fatal("TracerProvider() returned nil")
	}
	if _, ok := rt.MeterProvider().(noop.MeterProvider); !ok {
		t.Fatal("MeterProvider() should be noop when metrics are disabled")
	}
	if rt.TraceEnabled(InstrumentationRedis) {
		t.Fatal("TraceEnabled() should be false when tracing is disabled")
	}
	if rt.MetricsEnabled(InstrumentationHTTPServer) {
		t.Fatal("MetricsEnabled() should be false when metrics are disabled")
	}
}

func TestRuntimeInstrumentationOverrides(t *testing.T) {
	cfg := DefaultConfig("checkout", "2.0.0")
	cfg.Tracing.Enabled = true
	cfg.Metrics.Enabled = true
	cfg.Instrumentations.DefaultEnabled = true
	cfg.Instrumentations.Overrides = map[string]bool{
		InstrumentationRedis: false,
		"goim":               true,
	}

	rt, err := NewRuntime(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	defer rt.Shutdown(context.Background())

	if rt.TraceEnabled(InstrumentationRedis) {
		t.Fatal("redis tracing should be disabled by override")
	}
	if !rt.TraceEnabled(InstrumentationHTTPServer) {
		t.Fatal("http server tracing should use default enabled state")
	}
	if !rt.MetricsEnabled("goim") {
		t.Fatal("custom goim metrics should be enabled by override")
	}
}

func TestNewRuntimeSupportsMultipleExporters(t *testing.T) {
	cfg := DefaultConfig("catalog", "1.0.0")
	cfg.Tracing.Enabled = true
	cfg.Tracing.Exporters = []TraceExporterConfig{
		{Type: ExporterStdout},
		{Type: ExporterStdout},
	}
	cfg.Metrics.Enabled = true
	cfg.Metrics.Exporters = []MetricExporterConfig{
		{Type: ExporterStdout},
		{Type: ExporterStdout},
	}

	rt, err := NewRuntime(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	defer rt.Shutdown(context.Background())

	if rt.Tracer(InstrumentationHTTPServer) == nil {
		t.Fatal("Tracer() returned nil")
	}
	if rt.Meter(InstrumentationHTTPServer) == nil {
		t.Fatal("Meter() returned nil")
	}
}
