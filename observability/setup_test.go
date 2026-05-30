package observability

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/metric/noop"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

func TestNewFunctionsSupportLogOnlyService(t *testing.T) {
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
	cfg.ApplyDefaults()

	log, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer log.Close()

	res, err := NewResource(context.Background(), cfg.Service)
	if err != nil {
		t.Fatalf("NewResource() error = %v", err)
	}
	propagator := NewPropagator()
	tracerProvider, shutdownTrace, err := NewTracerProvider(context.Background(), cfg.Tracing, res)
	if err != nil {
		t.Fatalf("NewTracerProvider() error = %v", err)
	}
	meterProvider, shutdownMetrics, err := NewMeterProvider(context.Background(), cfg.Metrics, res)
	if err != nil {
		t.Fatalf("NewMeterProvider() error = %v", err)
	}
	defer shutdownTrace(context.Background())
	defer shutdownMetrics(context.Background())
	InstallGlobal(propagator, tracerProvider, meterProvider)

	if log == nil {
		t.Fatal("NewLogger() returned nil")
	}
	if tracerProvider == nil {
		t.Fatal("NewTracerProvider() returned nil")
	}
	if _, ok := meterProvider.(noop.MeterProvider); !ok {
		t.Fatal("NewMeterProvider() should return noop when metrics are disabled")
	}
	if cfg.TraceEnabled(InstrumentationRedis) {
		t.Fatal("TraceEnabled() should be false when tracing is disabled")
	}
	if cfg.MetricsEnabled(InstrumentationHTTPServer) {
		t.Fatal("MetricsEnabled() should be false when metrics are disabled")
	}
}

func TestConfigInstrumentationOverrides(t *testing.T) {
	cfg := DefaultConfig("checkout", "2.0.0")
	cfg.Tracing.Enabled = true
	cfg.Metrics.Enabled = true
	cfg.Instrumentations.DefaultEnabled = true
	cfg.Instrumentations.Overrides = map[string]bool{
		InstrumentationRedis: false,
		"goim":               true,
	}

	if cfg.TraceEnabled(InstrumentationRedis) {
		t.Fatal("redis tracing should be disabled by override")
	}
	if !cfg.TraceEnabled(InstrumentationHTTPServer) {
		t.Fatal("http server tracing should use default enabled state")
	}
	if !cfg.MetricsEnabled("goim") {
		t.Fatal("custom goim metrics should be enabled by override")
	}
}

func TestNewFunctionsSupportMultipleExporters(t *testing.T) {
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

	res, err := NewResource(context.Background(), cfg.Service)
	if err != nil {
		t.Fatalf("NewResource() error = %v", err)
	}
	tracerProvider, shutdownTrace, err := NewTracerProvider(context.Background(), cfg.Tracing, res)
	if err != nil {
		t.Fatalf("NewTracerProvider() error = %v", err)
	}
	defer shutdownTrace(context.Background())

	meterProvider, shutdownMetrics, err := NewMeterProvider(context.Background(), cfg.Metrics, res)
	if err != nil {
		t.Fatalf("NewMeterProvider() error = %v", err)
	}
	defer shutdownMetrics(context.Background())

	if tracerProvider.Tracer(InstrumentationHTTPServer) == nil {
		t.Fatal("TracerProvider.Tracer() returned nil")
	}
	if meterProvider.Meter(InstrumentationHTTPServer) == nil {
		t.Fatal("MeterProvider.Meter() returned nil")
	}
}
