package tracing

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.21.0"
)

// NewTracer 创建新的链路追踪器.
// ServiceName 和 ServiceVersion 从 cfg 中读取.
func NewTracer(cfg *TracingConfig) (*trace.TracerProvider, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}

	if !cfg.Enabled {
		// 返回无操作的追踪器
		return trace.NewTracerProvider(), nil
	}

	if cfg.ServiceName == "" {
		return nil, ErrEmptyServiceName
	}

	if cfg.OTLP == nil || cfg.OTLP.Endpoint == "" {
		return nil, ErrEmptyEndpoint
	}

	// 处理endpoint URL，移除协议前缀
	endpoint := cfg.OTLP.Endpoint
	if after, ok := strings.CutPrefix(endpoint, "http://"); ok {
		endpoint = after
	}
	if after, ok := strings.CutPrefix(endpoint, "https://"); ok {
		endpoint = after
	}

	// 根据 Protocol 创建不同的 OTLP 导出器
	var exp *otlptrace.Exporter
	var err error

	switch cfg.OTLP.Protocol {
	case "grpc":
		grpcOpts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(endpoint),
		}
		if cfg.OTLP.Insecure {
			grpcOpts = append(grpcOpts, otlptracegrpc.WithInsecure())
		}
		if len(cfg.OTLP.Headers) > 0 {
			grpcOpts = append(grpcOpts, otlptracegrpc.WithHeaders(cfg.OTLP.Headers))
		}
		exp, err = otlptracegrpc.New(context.Background(), grpcOpts...)
	default:
		httpOpts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(endpoint),
		}
		if cfg.OTLP.Insecure {
			httpOpts = append(httpOpts, otlptracehttp.WithInsecure())
		}
		if len(cfg.OTLP.Headers) > 0 {
			httpOpts = append(httpOpts, otlptracehttp.WithHeaders(cfg.OTLP.Headers))
		}
		exp, err = otlptracehttp.New(context.Background(), httpOpts...)
	}

	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCreateExporter, err)
	}

	// 创建资源
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCreateResource, err)
	}

	// 设置采样率，默认100%
	samplingRate := cfg.SamplingRate
	if samplingRate <= 0 || samplingRate > 1 {
		samplingRate = 1.0
	}

	// 创建TracerProvider
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exp),
		trace.WithResource(res),
		trace.WithSampler(trace.TraceIDRatioBased(samplingRate)),
	)

	// 设置全局TracerProvider
	otel.SetTracerProvider(tp)

	// 设置全局传播器
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}

// MustNewTracer 创建链路追踪器，失败时 panic.
func MustNewTracer(cfg *TracingConfig) *trace.TracerProvider {
	tp, err := NewTracer(cfg)
	if err != nil {
		panic(err)
	}
	return tp
}
