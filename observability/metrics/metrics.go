// Package metrics 提供 Prometheus 指标收集功能.
package metrics

import (
	"net/http"
	"time"
)

// Collector 指标收集器接口.
type Collector interface {
	// HTTP 指标
	RecordHTTPRequest(method, path, statusCode string, duration time.Duration, requestSize, responseSize float64)

	// gRPC 指标
	RecordGRPCRequest(method, service, statusCode string, duration time.Duration)

	// 系统指标
	RecordPanic(service, method, endpoint string)
	UpdateGoroutineCount(count int)
	UpdateMemoryUsage(bytes int64)

	// 自定义指标
	Counter(name string, labels map[string]string)
	Histogram(name string, value float64, labels map[string]string)
	Gauge(name string, value float64, labels map[string]string)

	// Handler
	GetHandler() http.Handler
	GetPath() string
}

// NewMetrics 创建指标收集器.
func NewMetrics(cfg *Config) (*PrometheusCollector, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}

	return NewPrometheus(cfg)
}

// MustNewMetrics 创建指标收集器，失败时 panic.
func MustNewMetrics(cfg *Config) *PrometheusCollector {
	c, err := NewMetrics(cfg)
	if err != nil {
		panic(err)
	}
	return c
}
