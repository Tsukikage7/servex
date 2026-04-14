package tracing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTracer_NilConfig(t *testing.T) {
	tp, err := NewTracer(nil)

	assert.Nil(t, tp)
	assert.ErrorIs(t, err, ErrNilConfig)
}

func TestNewTracer_Disabled(t *testing.T) {
	cfg := &TracingConfig{
		Enabled: false,
	}

	tp, err := NewTracer(cfg)

	require.NoError(t, err)
	assert.NotNil(t, tp)
	_ = tp.Shutdown(t.Context())
}

func TestNewTracer_EmptyServiceName(t *testing.T) {
	cfg := &TracingConfig{
		Enabled: true,
		OTLP: &OTLPConfig{
			Endpoint: "localhost:4318",
		},
	}

	tp, err := NewTracer(cfg)

	assert.Nil(t, tp)
	assert.ErrorIs(t, err, ErrEmptyServiceName)
}

func TestNewTracer_NilOTLP(t *testing.T) {
	cfg := &TracingConfig{
		Enabled:     true,
		ServiceName: "test-service",
		OTLP:        nil,
	}

	tp, err := NewTracer(cfg)

	assert.Nil(t, tp)
	assert.ErrorIs(t, err, ErrEmptyEndpoint)
}

func TestNewTracer_EmptyEndpoint(t *testing.T) {
	cfg := &TracingConfig{
		Enabled:     true,
		ServiceName: "test-service",
		OTLP: &OTLPConfig{
			Endpoint: "",
		},
	}

	tp, err := NewTracer(cfg)

	assert.Nil(t, tp)
	assert.ErrorIs(t, err, ErrEmptyEndpoint)
}

func TestNewTracer_Success(t *testing.T) {
	cfg := &TracingConfig{
		Enabled:        true,
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		SamplingRate:   0.5,
		OTLP: &OTLPConfig{
			Endpoint: "localhost:4318",
		},
	}

	tp, err := NewTracer(cfg)

	require.NoError(t, err)
	assert.NotNil(t, tp)
	_ = tp.Shutdown(t.Context())
}

func TestNewTracer_WithHttpPrefix(t *testing.T) {
	cfg := &TracingConfig{
		Enabled:     true,
		ServiceName: "test-service",
		OTLP: &OTLPConfig{
			Endpoint: "http://localhost:4318",
		},
	}

	tp, err := NewTracer(cfg)

	require.NoError(t, err)
	assert.NotNil(t, tp)
	_ = tp.Shutdown(t.Context())
}

func TestNewTracer_WithHttpsPrefix(t *testing.T) {
	cfg := &TracingConfig{
		Enabled:     true,
		ServiceName: "test-service",
		OTLP: &OTLPConfig{
			Endpoint: "https://localhost:4318",
		},
	}

	tp, err := NewTracer(cfg)

	require.NoError(t, err)
	assert.NotNil(t, tp)
	_ = tp.Shutdown(t.Context())
}

func TestNewTracer_WithHeaders(t *testing.T) {
	cfg := &TracingConfig{
		Enabled:     true,
		ServiceName: "test-service",
		OTLP: &OTLPConfig{
			Endpoint: "localhost:4318",
			Headers: map[string]string{
				"Authorization": "Bearer token",
			},
		},
	}

	tp, err := NewTracer(cfg)

	require.NoError(t, err)
	assert.NotNil(t, tp)
	_ = tp.Shutdown(t.Context())
}

func TestNewTracer_InvalidSamplingRate(t *testing.T) {
	tests := []struct {
		name         string
		samplingRate float64
	}{
		{"negative", -0.5},
		{"zero", 0},
		{"greater than 1", 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &TracingConfig{
				Enabled:      true,
				ServiceName:  "test-service",
				SamplingRate: tt.samplingRate,
				OTLP: &OTLPConfig{
					Endpoint: "localhost:4318",
				},
			}

			tp, err := NewTracer(cfg)

			// 无效采样率会被自动修正为1.0，不会报错
			require.NoError(t, err)
			assert.NotNil(t, tp)
			_ = tp.Shutdown(t.Context())
		})
	}
}

func TestNewTracer_ValidSamplingRate(t *testing.T) {
	cfg := &TracingConfig{
		Enabled:      true,
		ServiceName:  "test-service",
		SamplingRate: 0.1,
		OTLP: &OTLPConfig{
			Endpoint: "localhost:4318",
		},
	}

	tp, err := NewTracer(cfg)

	require.NoError(t, err)
	assert.NotNil(t, tp)
	_ = tp.Shutdown(t.Context())
}

func TestMustNewTracer_Success(t *testing.T) {
	cfg := &TracingConfig{
		Enabled:     true,
		ServiceName: "test-service",
		OTLP: &OTLPConfig{
			Endpoint: "localhost:4318",
		},
	}

	assert.NotPanics(t, func() {
		tp := MustNewTracer(cfg)
		assert.NotNil(t, tp)
		_ = tp.Shutdown(t.Context())
	})
}

func TestMustNewTracer_Panic(t *testing.T) {
	assert.Panics(t, func() {
		MustNewTracer(nil)
	})

	cfg := &TracingConfig{
		Enabled: true,
		OTLP: &OTLPConfig{
			Endpoint: "localhost:4318",
		},
	}
	assert.Panics(t, func() {
		MustNewTracer(cfg)
	})
}

func TestNewTracer_GRPCProtocol(t *testing.T) {
	cfg := &TracingConfig{
		Enabled:     true,
		ServiceName: "test-service",
		OTLP: &OTLPConfig{
			Endpoint: "localhost:4317",
			Protocol: "grpc",
			Insecure: true,
		},
	}

	tp, err := NewTracer(cfg)

	require.NoError(t, err)
	assert.NotNil(t, tp)
	_ = tp.Shutdown(t.Context())
}

func TestTracingConfig(t *testing.T) {
	cfg := &TracingConfig{
		Enabled:        true,
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		SamplingRate:   0.5,
		OTLP: &OTLPConfig{
			Endpoint: "localhost:4318",
			Headers: map[string]string{
				"key": "value",
			},
		},
	}

	assert.True(t, cfg.Enabled)
	assert.Equal(t, "test-service", cfg.ServiceName)
	assert.Equal(t, "1.0.0", cfg.ServiceVersion)
	assert.Equal(t, 0.5, cfg.SamplingRate)
	assert.Equal(t, "localhost:4318", cfg.OTLP.Endpoint)
	assert.Equal(t, "value", cfg.OTLP.Headers["key"])
}
