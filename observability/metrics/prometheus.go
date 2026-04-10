package metrics

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusCollector Prometheus 指标收集器实现.
type PrometheusCollector struct {
	config *Config

	// HTTP 指标
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	httpRequestSize     *prometheus.HistogramVec
	httpResponseSize    *prometheus.HistogramVec

	// gRPC 指标
	grpcRequestsTotal   *prometheus.CounterVec
	grpcRequestDuration *prometheus.HistogramVec

	// 系统指标
	goroutineCount prometheus.Gauge
	memoryUsage    prometheus.Gauge
	panicTotal     *prometheus.CounterVec

	// 自定义指标注册表
	counters   map[string]*prometheus.CounterVec
	histograms map[string]*prometheus.HistogramVec
	gauges     map[string]*prometheus.GaugeVec
	mu         sync.RWMutex

	registry *prometheus.Registry
}

// NewPrometheus 创建 Prometheus 指标收集器.
func NewPrometheus(cfg *Config) (*PrometheusCollector, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}

	namespace := cfg.Namespace
	if namespace == "" {
		namespace = "app"
	}

	// 创建新的注册表，避免与默认注册表冲突
	registry := prometheus.NewRegistry()

	c := &PrometheusCollector{
		config:     cfg,
		counters:   make(map[string]*prometheus.CounterVec),
		histograms: make(map[string]*prometheus.HistogramVec),
		gauges:     make(map[string]*prometheus.GaugeVec),
		registry:   registry,
	}

	// HTTP 指标
	c.httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status_code"},
	)

	c.httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	c.httpRequestSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "request_size_bytes",
			Help:      "HTTP request size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 5),
		},
		[]string{"method", "path"},
	)

	c.httpResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "response_size_bytes",
			Help:      "HTTP response size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 5),
		},
		[]string{"method", "path"},
	)

	// gRPC 指标
	c.grpcRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "grpc",
			Name:      "requests_total",
			Help:      "Total number of gRPC requests",
		},
		[]string{"method", "service", "status_code"},
	)

	c.grpcRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "grpc",
			Name:      "request_duration_seconds",
			Help:      "gRPC request duration in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "service"},
	)

	// 系统指标
	c.goroutineCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "system",
			Name:      "goroutines",
			Help:      "Number of goroutines",
		},
	)

	c.memoryUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "system",
			Name:      "memory_usage_bytes",
			Help:      "Memory usage in bytes",
		},
	)

	c.panicTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "system",
			Name:      "panic_total",
			Help:      "Total number of panics recovered",
		},
		[]string{"service", "method", "endpoint"},
	)

	// 注册所有指标
	collectors := []prometheus.Collector{
		c.httpRequestsTotal,
		c.httpRequestDuration,
		c.httpRequestSize,
		c.httpResponseSize,
		c.grpcRequestsTotal,
		c.grpcRequestDuration,
		c.goroutineCount,
		c.memoryUsage,
		c.panicTotal,
	}

	for _, collector := range collectors {
		if err := registry.Register(collector); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrRegisterMetric, err)
		}
	}

	return c, nil
}

// RecordHTTPRequest 记录 HTTP 请求指标.
func (c *PrometheusCollector) RecordHTTPRequest(method, path, statusCode string, duration time.Duration, requestSize, responseSize float64) {
	c.httpRequestsTotal.WithLabelValues(method, path, statusCode).Inc()
	c.httpRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
	c.httpRequestSize.WithLabelValues(method, path).Observe(requestSize)
	c.httpResponseSize.WithLabelValues(method, path).Observe(responseSize)
}

// RecordGRPCRequest 记录 gRPC 请求指标.
func (c *PrometheusCollector) RecordGRPCRequest(method, service, statusCode string, duration time.Duration) {
	c.grpcRequestsTotal.WithLabelValues(method, service, statusCode).Inc()
	c.grpcRequestDuration.WithLabelValues(method, service).Observe(duration.Seconds())
}

// RecordPanic 记录 panic 事件.
func (c *PrometheusCollector) RecordPanic(service, method, endpoint string) {
	c.panicTotal.WithLabelValues(service, method, endpoint).Inc()
}

// UpdateGoroutineCount 更新 goroutine 数量.
func (c *PrometheusCollector) UpdateGoroutineCount(count int) {
	c.goroutineCount.Set(float64(count))
}

// UpdateMemoryUsage 更新内存使用量.
func (c *PrometheusCollector) UpdateMemoryUsage(bytes int64) {
	c.memoryUsage.Set(float64(bytes))
}

// Counter 增加计数器.
// 使用示例:
//
//	collector.Counter("payment_failed_total", map[string]string{"channel": "alipay", "reason": "timeout"})
func (c *PrometheusCollector) Counter(name string, labels map[string]string) {
	// 提取 label 名称和值（保持顺序一致）
	labelNames, labelValues := extractLabels(labels)

	c.mu.RLock()
	counter, exists := c.counters[name]
	c.mu.RUnlock()

	if !exists {
		c.mu.Lock()
		// 双重检查
		if counter, exists = c.counters[name]; !exists {
			counter = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: c.config.Namespace,
					Name:      name,
					Help:      "Custom counter: " + name,
				},
				labelNames,
			)

			if err := c.registry.Register(counter); err == nil {
				c.counters[name] = counter
			} else {
				slog.Error("[metrics] 注册自定义 Counter 失败", slog.String("name", name), slog.Any("error", err))
			}
		}
		c.mu.Unlock()
	}

	if counter != nil {
		counter.WithLabelValues(labelValues...).Inc()
	}
}

// Histogram 观察自定义直方图.
// 使用示例:
//
//	collector.Histogram("payment_duration_seconds", 0.5, map[string]string{"channel": "alipay"})
func (c *PrometheusCollector) Histogram(name string, value float64, labels map[string]string) {
	// 提取 label 名称和值（保持顺序一致）
	labelNames, labelValues := extractLabels(labels)

	c.mu.RLock()
	histogram, exists := c.histograms[name]
	c.mu.RUnlock()

	if !exists {
		c.mu.Lock()
		// 双重检查
		if histogram, exists = c.histograms[name]; !exists {
			histogram = prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Namespace: c.config.Namespace,
					Name:      name,
					Help:      "Custom histogram: " + name,
					Buckets:   prometheus.DefBuckets,
				},
				labelNames,
			)

			if err := c.registry.Register(histogram); err == nil {
				c.histograms[name] = histogram
			} else {
				slog.Error("[metrics] 注册自定义 Histogram 失败", slog.String("name", name), slog.Any("error", err))
			}
		}
		c.mu.Unlock()
	}

	if histogram != nil {
		histogram.WithLabelValues(labelValues...).Observe(value)
	}
}

// Gauge 设置自定义仪表盘.
// 使用示例:
//
//	collector.Gauge("pending_orders", 42, map[string]string{"status": "unpaid"})
func (c *PrometheusCollector) Gauge(name string, value float64, labels map[string]string) {
	// 提取 label 名称和值（保持顺序一致）
	labelNames, labelValues := extractLabels(labels)

	c.mu.RLock()
	gauge, exists := c.gauges[name]
	c.mu.RUnlock()

	if !exists {
		c.mu.Lock()
		// 双重检查
		if gauge, exists = c.gauges[name]; !exists {
			gauge = prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: c.config.Namespace,
					Name:      name,
					Help:      "Custom gauge: " + name,
				},
				labelNames,
			)

			if err := c.registry.Register(gauge); err == nil {
				c.gauges[name] = gauge
			} else {
				slog.Error("[metrics] 注册自定义 Gauge 失败", slog.String("name", name), slog.Any("error", err))
			}
		}
		c.mu.Unlock()
	}

	if gauge != nil {
		gauge.WithLabelValues(labelValues...).Set(value)
	}
}

// extractLabels 从 map 中提取 label 名称和值，确保顺序一致.
// 通过排序 key 来保证每次调用的顺序稳定.
func extractLabels(labels map[string]string) ([]string, []string) {
	labelNames := make([]string, 0, len(labels))
	for k := range labels {
		labelNames = append(labelNames, k)
	}
	sort.Strings(labelNames)

	labelValues := make([]string, 0, len(labels))
	for _, k := range labelNames {
		labelValues = append(labelValues, labels[k])
	}

	return labelNames, labelValues
}

// GetHandler 返回 metrics 的 HTTP 处理器.
func (c *PrometheusCollector) GetHandler() http.Handler {
	return promhttp.HandlerFor(c.registry, promhttp.HandlerOpts{})
}

// GetPath 返回 metrics 路径.
func (c *PrometheusCollector) GetPath() string {
	if c.config.Path == "" {
		return "/metrics"
	}
	return c.config.Path
}
