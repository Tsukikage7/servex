package observability

import (
	"context"
	"fmt"
	"strings"

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

// Runtime 持有服务的可观测性运行时依赖.
type Runtime struct {
	config         Config
	logger         logger.Logger
	propagator     propagation.TextMapPropagator
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
	traceSDK       *sdktrace.TracerProvider
	metricSDK      *sdkmetric.MeterProvider
}

// NewRuntime 根据配置创建可观测性运行时.
func NewRuntime(ctx context.Context, cfg Config) (*Runtime, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg.ApplyDefaults()

	log, err := logger.NewLogger(&cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("observability: 创建日志失败: %w", err)
	}

	res, err := newResource(ctx, cfg.Service)
	if err != nil {
		_ = log.Close()
		return nil, err
	}

	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)

	traceProvider, traceSDK, err := newTracerProvider(ctx, cfg.Tracing, res)
	if err != nil {
		_ = log.Close()
		return nil, err
	}

	meterProvider, metricSDK, err := newMeterProvider(ctx, cfg.Metrics, res)
	if err != nil {
		if traceSDK != nil {
			_ = traceSDK.Shutdown(ctx)
		}
		_ = log.Close()
		return nil, err
	}

	otel.SetTextMapPropagator(propagator)
	otel.SetTracerProvider(traceProvider)
	otel.SetMeterProvider(meterProvider)

	return &Runtime{
		config:         cfg,
		logger:         log,
		propagator:     propagator,
		tracerProvider: traceProvider,
		meterProvider:  meterProvider,
		traceSDK:       traceSDK,
		metricSDK:      metricSDK,
	}, nil
}

// MustNewRuntime 创建 Runtime，失败时 panic.
func MustNewRuntime(ctx context.Context, cfg Config) *Runtime {
	rt, err := NewRuntime(ctx, cfg)
	if err != nil {
		panic(err)
	}
	return rt
}

// Logger 返回结构化日志.
func (r *Runtime) Logger() logger.Logger {
	if r == nil || r.logger == nil {
		return logger.Nop()
	}
	return r.logger
}

// TracerProvider 返回 trace provider.
func (r *Runtime) TracerProvider() trace.TracerProvider {
	if r == nil || r.tracerProvider == nil {
		return tracenoop.NewTracerProvider()
	}
	return r.tracerProvider
}

// MeterProvider 返回 meter provider.
func (r *Runtime) MeterProvider() metric.MeterProvider {
	if r == nil || r.meterProvider == nil {
		return metricnoop.NewMeterProvider()
	}
	return r.meterProvider
}

// Propagator 返回文本传播器.
func (r *Runtime) Propagator() propagation.TextMapPropagator {
	if r == nil || r.propagator == nil {
		return propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	}
	return r.propagator
}

// Tracer 返回指定 instrumentation scope 的 tracer.
func (r *Runtime) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	return r.TracerProvider().Tracer(name, opts...)
}

// Meter 返回指定 instrumentation scope 的 meter.
func (r *Runtime) Meter(name string, opts ...metric.MeterOption) metric.Meter {
	return r.MeterProvider().Meter(name, opts...)
}

// InstrumentationEnabled 判断组件埋点是否启用.
func (r *Runtime) InstrumentationEnabled(name string) bool {
	if r == nil {
		return false
	}
	return r.config.Instrumentations.Enabled(name)
}

// TraceEnabled 判断组件 trace 是否启用.
func (r *Runtime) TraceEnabled(name string) bool {
	if r == nil || !r.config.Tracing.Enabled {
		return false
	}
	return r.InstrumentationEnabled(name)
}

// MetricsEnabled 判断组件 metrics 是否启用.
func (r *Runtime) MetricsEnabled(name string) bool {
	if r == nil || !r.config.Metrics.Enabled {
		return false
	}
	return r.InstrumentationEnabled(name)
}

// Shutdown 关闭 Runtime 持有的可观测性资源.
func (r *Runtime) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var err error
	if r.traceSDK != nil {
		err = joinError(err, r.traceSDK.Shutdown(ctx))
	}
	if r.metricSDK != nil {
		err = joinError(err, r.metricSDK.Shutdown(ctx))
	}
	if r.logger != nil {
		err = joinError(err, r.logger.Close())
	}
	return err
}

func newResource(ctx context.Context, cfg ServiceConfig) (*resource.Resource, error) {
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

func newTracerProvider(ctx context.Context, cfg TracingConfig, res *resource.Resource) (trace.TracerProvider, *sdktrace.TracerProvider, error) {
	if !cfg.Enabled {
		return tracenoop.NewTracerProvider(), nil, nil
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

func joinError(current, next error) error {
	if current == nil {
		return next
	}
	if next == nil {
		return current
	}
	return fmt.Errorf("%w; %w", current, next)
}
