package metrics

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// OTelOption 配置 OTelCollector 的选项函数.
type OTelOption func(*otelOptions)

type otelOptions struct {
	meterProvider metric.MeterProvider
	serviceName   string
	exporter      sdkmetric.Exporter
}

// WithMeterProvider 指定自定义 MeterProvider.
func WithMeterProvider(mp metric.MeterProvider) OTelOption {
	return func(o *otelOptions) {
		o.meterProvider = mp
	}
}

// WithServiceName 指定服务名称，用于 Meter 的 instrumentation scope.
func WithServiceName(name string) OTelOption {
	return func(o *otelOptions) {
		o.serviceName = name
	}
}

// WithExporter 指定指标导出器，用于构建内置 MeterProvider.
// 若同时设置了 WithMeterProvider，则 Exporter 会被忽略.
func WithExporter(exp sdkmetric.Exporter) OTelOption {
	return func(o *otelOptions) {
		o.exporter = exp
	}
}

// OTelCollector 基于 OpenTelemetry Metrics SDK 的指标收集器.
type OTelCollector struct {
	config *Config
	meter  metric.Meter

	// HTTP 指标
	httpRequestsTotal   metric.Int64Counter
	httpRequestDuration metric.Float64Histogram
	httpRequestSize     metric.Float64Histogram
	httpResponseSize    metric.Float64Histogram

	// gRPC 指标
	grpcRequestsTotal   metric.Int64Counter
	grpcRequestDuration metric.Float64Histogram

	// 系统指标
	goroutineCount metric.Int64Gauge
	memoryUsage    metric.Int64Gauge
	panicTotal     metric.Int64Counter

	// 自定义指标注册表
	counters   map[string]metric.Int64Counter
	histograms map[string]metric.Float64Histogram
	gauges     map[string]metric.Float64Gauge
	mu         sync.RWMutex
}

// NewOTel 创建 OpenTelemetry 指标收集器.
func NewOTel(cfg *Config, opts ...OTelOption) (*OTelCollector, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}

	o := &otelOptions{
		serviceName: "app",
	}
	for _, opt := range opts {
		opt(o)
	}

	// 确定 MeterProvider
	mp := o.meterProvider
	if mp == nil {
		if o.exporter != nil {
			reader := sdkmetric.NewPeriodicReader(o.exporter)
			mp = sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		} else {
			mp = otel.GetMeterProvider()
		}
	}

	namespace := cfg.Namespace
	if namespace == "" {
		namespace = "app"
	}

	// 用 serviceName 作为 instrumentation scope
	meter := mp.Meter(o.serviceName)

	c := &OTelCollector{
		config:     cfg,
		meter:      meter,
		counters:   make(map[string]metric.Int64Counter),
		histograms: make(map[string]metric.Float64Histogram),
		gauges:     make(map[string]metric.Float64Gauge),
	}

	var err error

	// HTTP 指标
	c.httpRequestsTotal, err = meter.Int64Counter(
		namespace+".http.requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return nil, err
	}

	c.httpRequestDuration, err = meter.Float64Histogram(
		namespace+".http.request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
	)
	if err != nil {
		return nil, err
	}

	c.httpRequestSize, err = meter.Float64Histogram(
		namespace+".http.request_size_bytes",
		metric.WithDescription("HTTP request size in bytes"),
	)
	if err != nil {
		return nil, err
	}

	c.httpResponseSize, err = meter.Float64Histogram(
		namespace+".http.response_size_bytes",
		metric.WithDescription("HTTP response size in bytes"),
	)
	if err != nil {
		return nil, err
	}

	// gRPC 指标
	c.grpcRequestsTotal, err = meter.Int64Counter(
		namespace+".grpc.requests_total",
		metric.WithDescription("Total number of gRPC requests"),
	)
	if err != nil {
		return nil, err
	}

	c.grpcRequestDuration, err = meter.Float64Histogram(
		namespace+".grpc.request_duration_seconds",
		metric.WithDescription("gRPC request duration in seconds"),
	)
	if err != nil {
		return nil, err
	}

	// 系统指标
	c.goroutineCount, err = meter.Int64Gauge(
		namespace+".system.goroutines",
		metric.WithDescription("Number of goroutines"),
	)
	if err != nil {
		return nil, err
	}

	c.memoryUsage, err = meter.Int64Gauge(
		namespace+".system.memory_usage_bytes",
		metric.WithDescription("Memory usage in bytes"),
	)
	if err != nil {
		return nil, err
	}

	c.panicTotal, err = meter.Int64Counter(
		namespace+".system.panic_total",
		metric.WithDescription("Total number of panics recovered"),
	)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// RecordHTTPRequest 记录 HTTP 请求指标.
func (c *OTelCollector) RecordHTTPRequest(method, path, statusCode string, duration time.Duration, requestSize, responseSize float64) {
	ctx := context.Background()
	attrs := metric.WithAttributes(
		attribute.String("method", method),
		attribute.String("path", path),
		attribute.String("status_code", statusCode),
	)
	c.httpRequestsTotal.Add(ctx, 1, attrs)

	pathAttrs := metric.WithAttributes(
		attribute.String("method", method),
		attribute.String("path", path),
	)
	c.httpRequestDuration.Record(ctx, duration.Seconds(), pathAttrs)
	c.httpRequestSize.Record(ctx, requestSize, pathAttrs)
	c.httpResponseSize.Record(ctx, responseSize, pathAttrs)
}

// RecordGRPCRequest 记录 gRPC 请求指标.
func (c *OTelCollector) RecordGRPCRequest(method, service, statusCode string, duration time.Duration) {
	ctx := context.Background()
	attrs := metric.WithAttributes(
		attribute.String("method", method),
		attribute.String("service", service),
		attribute.String("status_code", statusCode),
	)
	c.grpcRequestsTotal.Add(ctx, 1, attrs)

	svcAttrs := metric.WithAttributes(
		attribute.String("method", method),
		attribute.String("service", service),
	)
	c.grpcRequestDuration.Record(ctx, duration.Seconds(), svcAttrs)
}

// RecordPanic 记录 panic 事件.
func (c *OTelCollector) RecordPanic(service, method, endpoint string) {
	ctx := context.Background()
	attrs := metric.WithAttributes(
		attribute.String("service", service),
		attribute.String("method", method),
		attribute.String("endpoint", endpoint),
	)
	c.panicTotal.Add(ctx, 1, attrs)
}

// UpdateGoroutineCount 更新 goroutine 数量.
func (c *OTelCollector) UpdateGoroutineCount(count int) {
	c.goroutineCount.Record(context.Background(), int64(count))
}

// UpdateMemoryUsage 更新内存使用量.
func (c *OTelCollector) UpdateMemoryUsage(bytes int64) {
	c.memoryUsage.Record(context.Background(), bytes)
}

// Counter 增加计数器.
func (c *OTelCollector) Counter(name string, labels map[string]string) {
	counter := c.getOrCreateCounter(name)
	if counter != nil {
		counter.Add(context.Background(), 1, metric.WithAttributes(mapToAttributes(labels)...))
	}
}

// Histogram 观察自定义直方图.
func (c *OTelCollector) Histogram(name string, value float64, labels map[string]string) {
	hist := c.getOrCreateHistogram(name)
	if hist != nil {
		hist.Record(context.Background(), value, metric.WithAttributes(mapToAttributes(labels)...))
	}
}

// Gauge 设置自定义仪表盘.
func (c *OTelCollector) Gauge(name string, value float64, labels map[string]string) {
	gauge := c.getOrCreateGauge(name)
	if gauge != nil {
		gauge.Record(context.Background(), value, metric.WithAttributes(mapToAttributes(labels)...))
	}
}

// GetHandler 返回 metrics 的 HTTP 处理器.
// OTel 通常由 exporter 推送，此处返回空 handler 保持接口兼容.
func (c *OTelCollector) GetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("OTel metrics are exported via configured exporter, not pull-based.\n"))
	})
}

// GetPath 返回 metrics 路径.
func (c *OTelCollector) GetPath() string {
	if c.config.Path == "" {
		return "/metrics"
	}
	return c.config.Path
}

// getOrCreateCounter 获取或创建自定义计数器（线程安全）.
func (c *OTelCollector) getOrCreateCounter(name string) metric.Int64Counter {
	c.mu.RLock()
	counter, exists := c.counters[name]
	c.mu.RUnlock()

	if exists {
		return counter
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 双重检查
	if counter, exists = c.counters[name]; exists {
		return counter
	}

	counter, err := c.meter.Int64Counter(
		c.config.Namespace+"."+name,
		metric.WithDescription("Custom counter: "+name),
	)
	if err != nil {
		return nil
	}
	c.counters[name] = counter
	return counter
}

// getOrCreateHistogram 获取或创建自定义直方图（线程安全）.
func (c *OTelCollector) getOrCreateHistogram(name string) metric.Float64Histogram {
	c.mu.RLock()
	hist, exists := c.histograms[name]
	c.mu.RUnlock()

	if exists {
		return hist
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if hist, exists = c.histograms[name]; exists {
		return hist
	}

	hist, err := c.meter.Float64Histogram(
		c.config.Namespace+"."+name,
		metric.WithDescription("Custom histogram: "+name),
	)
	if err != nil {
		return nil
	}
	c.histograms[name] = hist
	return hist
}

// getOrCreateGauge 获取或创建自定义仪表盘（线程安全）.
func (c *OTelCollector) getOrCreateGauge(name string) metric.Float64Gauge {
	c.mu.RLock()
	gauge, exists := c.gauges[name]
	c.mu.RUnlock()

	if exists {
		return gauge
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if gauge, exists = c.gauges[name]; exists {
		return gauge
	}

	gauge, err := c.meter.Float64Gauge(
		c.config.Namespace+"."+name,
		metric.WithDescription("Custom gauge: "+name),
	)
	if err != nil {
		return nil
	}
	c.gauges[name] = gauge
	return gauge
}

// mapToAttributes 将 map[string]string 转为 OTel attribute 切片，按 key 排序确保稳定性.
func mapToAttributes(labels map[string]string) []attribute.KeyValue {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	attrs := make([]attribute.KeyValue, 0, len(labels))
	for _, k := range keys {
		attrs = append(attrs, attribute.String(k, labels[k]))
	}
	return attrs
}
