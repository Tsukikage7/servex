package observability

import "testing"

func TestConfigApplyDefaultsSetsLoggerServiceName(t *testing.T) {
	cfg := Config{
		Service: ServiceConfig{
			Name:    "billing",
			Version: "1.2.3",
		},
	}

	cfg.ApplyDefaults()

	if cfg.Logger.ServiceName != "billing" {
		t.Fatalf("Logger.ServiceName = %q, want billing", cfg.Logger.ServiceName)
	}
	if cfg.Tracing.SamplingRate != 1 {
		t.Fatalf("Tracing.SamplingRate = %v, want 1", cfg.Tracing.SamplingRate)
	}
	if cfg.Metrics.Interval <= 0 {
		t.Fatal("Metrics.Interval should have a positive default")
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
