package observability

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// ShutdownFunc 关闭一个可观测性组件持有的资源.
type ShutdownFunc func(context.Context) error

// NewLogger 根据可观测性配置创建日志.
func NewLogger(cfg Config) (logger.Logger, error) {
	cfg.ApplyDefaults()
	log, err := logger.NewLogger(&cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("observability: 创建日志失败: %w", err)
	}
	return log, nil
}

// NewResource 根据服务身份创建 OpenTelemetry resource.
func NewResource(ctx context.Context, cfg ServiceConfig) (*resource.Resource, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	attrs := []attribute.KeyValue{
		semconv.ServiceName(cfg.Name),
		semconv.ServiceVersion(cfg.Version),
	}
	if cfg.Namespace != "" {
		attrs = append(attrs, semconv.ServiceNamespace(cfg.Namespace))
	}
	if cfg.Environment != "" {
		attrs = append(attrs, semconv.DeploymentEnvironmentName(cfg.Environment))
	}
	res, err := resource.New(ctx, resource.WithAttributes(attrs...))
	if err != nil {
		return nil, fmt.Errorf("observability: 创建资源失败: %w", err)
	}
	return res, nil
}

// NewPropagator 创建默认文本传播器.
func NewPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

// NewTracerProvider 根据配置创建 trace provider.
func NewTracerProvider(ctx context.Context, cfg TracingConfig, res *resource.Resource) (trace.TracerProvider, ShutdownFunc, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	tp, sdk, err := newTracerProvider(ctx, cfg, res)
	if err != nil {
		return nil, nil, err
	}
	return tp, shutdownTracerProvider(sdk), nil
}

// NewMeterProvider 根据配置创建 meter provider.
func NewMeterProvider(ctx context.Context, cfg MetricsConfig, res *resource.Resource) (metric.MeterProvider, ShutdownFunc, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	mp, sdk, err := newMeterProvider(ctx, cfg, res)
	if err != nil {
		return nil, nil, err
	}
	return mp, shutdownMeterProvider(sdk), nil
}

// InstallGlobal 设置 OpenTelemetry 全局 provider.
func InstallGlobal(propagator propagation.TextMapPropagator, tracerProvider trace.TracerProvider, meterProvider metric.MeterProvider) {
	if propagator == nil {
		propagator = NewPropagator()
	}
	if tracerProvider == nil {
		tracerProvider = tracenoop.NewTracerProvider()
	}
	if meterProvider == nil {
		meterProvider = metricnoop.NewMeterProvider()
	}
	otel.SetTextMapPropagator(propagator)
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
}

func newTracerProvider(ctx context.Context, cfg TracingConfig, res *resource.Resource) (trace.TracerProvider, *sdktrace.TracerProvider, error) {
	if !cfg.Enabled {
		return tracenoop.NewTracerProvider(), nil, nil
	}
	if cfg.SamplingRate <= 0 || cfg.SamplingRate > 1 {
		cfg.SamplingRate = 1
	}
	if res == nil {
		res = resource.Default()
	}

	options := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplingRate))),
	}
	for _, exporter := range cfg.Exporters {
		exp, err := newTraceExporter(ctx, exporter)
		if err != nil {
			return nil, nil, err
		}
		options = append(options, sdktrace.WithBatcher(exp))
	}

	tp := sdktrace.NewTracerProvider(options...)
	return tp, tp, nil
}

func newMeterProvider(ctx context.Context, cfg MetricsConfig, res *resource.Resource) (metric.MeterProvider, *sdkmetric.MeterProvider, error) {
	if !cfg.Enabled {
		return metricnoop.NewMeterProvider(), nil, nil
	}
	if cfg.Interval <= 0 {
		cfg.Interval = time.Minute
	}
	if res == nil {
		res = resource.Default()
	}

	options := []sdkmetric.Option{sdkmetric.WithResource(res)}
	for _, exporter := range cfg.Exporters {
		exp, err := newMetricExporter(ctx, exporter)
		if err != nil {
			return nil, nil, err
		}
		options = append(options, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp, sdkmetric.WithInterval(cfg.Interval))))
	}

	mp := sdkmetric.NewMeterProvider(options...)
	return mp, mp, nil
}

func newTraceExporter(ctx context.Context, cfg TraceExporterConfig) (sdktrace.SpanExporter, error) {
	switch strings.ToLower(cfg.Type) {
	case ExporterStdout:
		exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("observability: 创建 stdout trace exporter 失败: %w", err)
		}
		return exp, nil
	case ExporterOTLP, "":
		switch strings.ToLower(cfg.Protocol) {
		case ProtocolGRPC:
			opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(stripEndpointScheme(cfg.Endpoint))}
			if cfg.Insecure {
				opts = append(opts, otlptracegrpc.WithInsecure())
			}
			if len(cfg.Headers) > 0 {
				opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
			}
			exp, err := otlptracegrpc.New(ctx, opts...)
			if err != nil {
				return nil, fmt.Errorf("observability: 创建 OTLP gRPC trace exporter 失败: %w", err)
			}
			return exp, nil
		default:
			opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(stripEndpointScheme(cfg.Endpoint))}
			if cfg.Insecure {
				opts = append(opts, otlptracehttp.WithInsecure())
			}
			if len(cfg.Headers) > 0 {
				opts = append(opts, otlptracehttp.WithHeaders(cfg.Headers))
			}
			exp, err := otlptracehttp.New(ctx, opts...)
			if err != nil {
				return nil, fmt.Errorf("observability: 创建 OTLP HTTP trace exporter 失败: %w", err)
			}
			return exp, nil
		}
	default:
		return nil, fmt.Errorf("observability: 不支持的 trace exporter: %s", cfg.Type)
	}
}

func newMetricExporter(ctx context.Context, cfg MetricExporterConfig) (sdkmetric.Exporter, error) {
	switch strings.ToLower(cfg.Type) {
	case ExporterStdout:
		exp, err := stdoutmetric.New(stdoutmetric.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("observability: 创建 stdout metric exporter 失败: %w", err)
		}
		return exp, nil
	case ExporterOTLP, "":
		switch strings.ToLower(cfg.Protocol) {
		case ProtocolGRPC:
			opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(stripEndpointScheme(cfg.Endpoint))}
			if cfg.Insecure {
				opts = append(opts, otlpmetricgrpc.WithInsecure())
			}
			if len(cfg.Headers) > 0 {
				opts = append(opts, otlpmetricgrpc.WithHeaders(cfg.Headers))
			}
			exp, err := otlpmetricgrpc.New(ctx, opts...)
			if err != nil {
				return nil, fmt.Errorf("observability: 创建 OTLP gRPC metric exporter 失败: %w", err)
			}
			return exp, nil
		default:
			opts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(stripEndpointScheme(cfg.Endpoint))}
			if cfg.Insecure {
				opts = append(opts, otlpmetrichttp.WithInsecure())
			}
			if len(cfg.Headers) > 0 {
				opts = append(opts, otlpmetrichttp.WithHeaders(cfg.Headers))
			}
			exp, err := otlpmetrichttp.New(ctx, opts...)
			if err != nil {
				return nil, fmt.Errorf("observability: 创建 OTLP HTTP metric exporter 失败: %w", err)
			}
			return exp, nil
		}
	default:
		return nil, fmt.Errorf("observability: 不支持的 metric exporter: %s", cfg.Type)
	}
}

func stripEndpointScheme(endpoint string) string {
	endpoint = strings.TrimPrefix(endpoint, "http://")
	return strings.TrimPrefix(endpoint, "https://")
}

func shutdownTracerProvider(tp *sdktrace.TracerProvider) ShutdownFunc {
	return func(ctx context.Context) error {
		if tp == nil {
			return nil
		}
		if ctx == nil {
			ctx = context.Background()
		}
		return tp.Shutdown(ctx)
	}
}

func shutdownMeterProvider(mp *sdkmetric.MeterProvider) ShutdownFunc {
	return func(ctx context.Context) error {
		if mp == nil {
			return nil
		}
		if ctx == nil {
			ctx = context.Background()
		}
		return mp.Shutdown(ctx)
	}
}
