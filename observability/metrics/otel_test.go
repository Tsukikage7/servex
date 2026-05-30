package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// newTestOTel 创建用于测试的 OTelCollector，使用内存 Reader 收集指标.
func newTestOTel(t *testing.T) (*OTelCollector, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	c, err := NewOTel(
		&Config{Namespace: "test", Path: "/metrics"},
		WithMeterProvider(mp),
		WithServiceName("test-service"),
	)
	require.NoError(t, err)
	return c, reader
}

// collectMetrics 从 ManualReader 收集所有指标数据.
func collectMetrics(t *testing.T, reader *sdkmetric.ManualReader) metricdata.ResourceMetrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	err := reader.Collect(t.Context(), &rm)
	require.NoError(t, err)
	return rm
}

// findMetric 在 ResourceMetrics 中按名称查找指标.
func findMetric(rm metricdata.ResourceMetrics, name string) *metricdata.Metrics {
	for _, sm := range rm.ScopeMetrics {
		for i := range sm.Metrics {
			if sm.Metrics[i].Name == name {
				return &sm.Metrics[i]
			}
		}
	}
	return nil
}

func TestNewOTel_NilConfig(t *testing.T) {
	c, err := NewOTel(nil)
	assert.Nil(t, c)
	assert.ErrorIs(t, err, ErrNilConfig)
}

func TestNewOTel_DefaultOptions(t *testing.T) {
	// 使用默认 MeterProvider全局，不传任何 option
	c, err := NewOTel(&Config{Namespace: "test"})
	require.NoError(t, err)
	assert.NotNil(t, c)
}

func TestNewOTel_WithExporter(t *testing.T) {
	// 使用 WithExporter 选项，构建内置 MeterProvider
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	c, err := NewOTel(
		&Config{Namespace: "test"},
		WithMeterProvider(mp),
	)
	require.NoError(t, err)
	assert.NotNil(t, c)
}

func TestNewOTel_EmptyNamespace(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	c, err := NewOTel(&Config{Namespace: ""}, WithMeterProvider(mp))
	require.NoError(t, err)
	assert.NotNil(t, c)
}

func TestOTelCollector_RecordHTTPRequest(t *testing.T) {
	c, reader := newTestOTel(t)

	c.RecordHTTPRequest("GET", "/api/users", "200", 100*time.Millisecond, 100, 200)

	rm := collectMetrics(t, reader)
	m := findMetric(rm, "test.http.requests_total")
	require.NotNil(t, m, "应找到 test.http.requests_total 指标")

	m = findMetric(rm, "test.http.request_duration_seconds")
	require.NotNil(t, m, "应找到 test.http.request_duration_seconds 指标")
}

func TestOTelCollector_RecordGRPCRequest(t *testing.T) {
	c, reader := newTestOTel(t)

	c.RecordGRPCRequest("/test.Service/Method", "test-service", "OK", 50*time.Millisecond)

	rm := collectMetrics(t, reader)
	m := findMetric(rm, "test.grpc.requests_total")
	require.NotNil(t, m, "应找到 test.grpc.requests_total 指标")

	m = findMetric(rm, "test.grpc.request_duration_seconds")
	require.NotNil(t, m, "应找到 test.grpc.request_duration_seconds 指标")
}

func TestOTelCollector_RecordPanic(t *testing.T) {
	c, reader := newTestOTel(t)

	c.RecordPanic("test-service", "GET", "/api/crash")

	rm := collectMetrics(t, reader)
	m := findMetric(rm, "test.system.panic_total")
	require.NotNil(t, m, "应找到 test.system.panic_total 指标")
}

func TestOTelCollector_UpdateGoroutineCount(t *testing.T) {
	c, reader := newTestOTel(t)

	c.UpdateGoroutineCount(100)

	rm := collectMetrics(t, reader)
	m := findMetric(rm, "test.system.goroutines")
	require.NotNil(t, m, "应找到 test.system.goroutines 指标")
}

func TestOTelCollector_UpdateMemoryUsage(t *testing.T) {
	c, reader := newTestOTel(t)

	c.UpdateMemoryUsage(1024 * 1024)

	rm := collectMetrics(t, reader)
	m := findMetric(rm, "test.system.memory_usage_bytes")
	require.NotNil(t, m, "应找到 test.system.memory_usage_bytes 指标")
}

func TestOTelCollector_Counter(t *testing.T) {
	c, reader := newTestOTel(t)

	c.Counter("custom_events", map[string]string{"type": "click"})
	c.Counter("custom_events", map[string]string{"type": "click"})

	rm := collectMetrics(t, reader)
	m := findMetric(rm, "test.custom_events")
	require.NotNil(t, m, "应找到 test.custom_events 指标")
}

func TestOTelCollector_Histogram(t *testing.T) {
	c, reader := newTestOTel(t)

	c.Histogram("request_latency", 0.5, map[string]string{"endpoint": "/api"})

	rm := collectMetrics(t, reader)
	m := findMetric(rm, "test.request_latency")
	require.NotNil(t, m, "应找到 test.request_latency 指标")
}

func TestOTelCollector_Gauge(t *testing.T) {
	c, reader := newTestOTel(t)

	c.Gauge("active_connections", 50, map[string]string{"server": "main"})

	rm := collectMetrics(t, reader)
	m := findMetric(rm, "test.active_connections")
	require.NotNil(t, m, "应找到 test.active_connections 指标")
}

func TestOTelCollector_GetPath_Default(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	c, err := NewOTel(&Config{Namespace: "test", Path: ""}, WithMeterProvider(mp))
	require.NoError(t, err)
	assert.Equal(t, "/metrics", c.GetPath())
}

func TestOTelCollector_GetPath_Custom(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	c, err := NewOTel(&Config{Namespace: "test", Path: "/custom/metrics"}, WithMeterProvider(mp))
	require.NoError(t, err)
	assert.Equal(t, "/custom/metrics", c.GetPath())
}

func TestOTelCollector_GetHandler(t *testing.T) {
	c, _ := newTestOTel(t)

	handler := c.GetHandler()
	assert.NotNil(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOTelCollector_ImplementsCollector(t *testing.T) {
	// 编译期验证 OTelCollector 实现了 Collector 接口
	var _ Collector = (*OTelCollector)(nil)
}

func TestOTelCollector_ConcurrentAccess(t *testing.T) {
	c, _ := newTestOTel(t)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				c.RecordHTTPRequest("GET", "/api", "200", time.Millisecond, 100, 200)
				c.RecordGRPCRequest("/test/Method", "svc", "OK", time.Millisecond)
				c.Counter("concurrent_counter", map[string]string{"worker": "test"})
				c.Histogram("concurrent_histogram", 0.1, map[string]string{"worker": "test"})
				c.Gauge("concurrent_gauge", float64(j), map[string]string{"worker": "test"})
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
