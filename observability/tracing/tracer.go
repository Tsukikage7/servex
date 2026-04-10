package tracing

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// NewTracer 创建新的链路追踪器.
func NewTracer(cfg *TracingConfig, serviceName, serviceVersion string) (*trace.TracerProvider, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}

	if !cfg.Enabled {
		// 返回无操作的追踪器
		return trace.NewTracerProvider(), nil
	}

	if serviceName == "" {
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

	// 创建OTLP HTTP导出器选项
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(), // 使用HTTP而不是HTTPS
	}

	// 添加请求头
	if len(cfg.OTLP.Headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(cfg.OTLP.Headers))
	}

	// 创建OTLP导出器
	exp, err := otlptracehttp.New(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCreateExporter, err)
	}

	// 创建资源
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
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
func MustNewTracer(cfg *TracingConfig, serviceName, serviceVersion string) *trace.TracerProvider {
	tp, err := NewTracer(cfg, serviceName, serviceVersion)
	if err != nil {
		panic(err)
	}
	return tp
}
