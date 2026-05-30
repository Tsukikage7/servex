package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCounter(t *testing.T) {
	cfg := &Config{Namespace: "test"}
	collector, err := NewPrometheus(cfg)
	require.NoError(t, err)

	// 使用计数器自动注册
	collector.Counter("payment_failed_total", map[string]string{"channel": "alipay", "reason": "timeout"})
	collector.Counter("payment_failed_total", map[string]string{"channel": "wechat", "reason": "insufficient_balance"})

	// 验证指标
	body := getMetrics(t, collector)
	assert.Contains(t, body, "test_payment_failed_total")
	assert.Contains(t, body, `channel="alipay"`)
	assert.Contains(t, body, `reason="timeout"`)
}

func TestHistogram(t *testing.T) {
	cfg := &Config{Namespace: "test"}
	collector, err := NewPrometheus(cfg)
	require.NoError(t, err)

	// 使用直方图自动注册
	collector.Histogram("payment_amount", 99.9, map[string]string{"channel": "alipay"})
	collector.Histogram("payment_amount", 199.9, map[string]string{"channel": "wechat"})

	// 验证指标
	body := getMetrics(t, collector)
	assert.Contains(t, body, "test_payment_amount")
}

func TestGauge(t *testing.T) {
	cfg := &Config{Namespace: "test"}
	collector, err := NewPrometheus(cfg)
	require.NoError(t, err)

	// 使用仪表盘自动注册
	collector.Gauge("pending_orders", 42, map[string]string{"status": "unpaid"})
	collector.Gauge("pending_orders", 10, map[string]string{"status": "paid"})

	// 验证指标
	body := getMetrics(t, collector)
	assert.Contains(t, body, "test_pending_orders")
	assert.Contains(t, body, `status="unpaid"`)
}

func TestBusinessMetrics_PaymentExample(t *testing.T) {
	cfg := &Config{Namespace: "payment_service"}
	collector, err := NewPrometheus(cfg)
	require.NoError(t, err)

	// 模拟支付业务
	// 支付成功
	collector.Counter("payment_success_total", map[string]string{"channel": "alipay"})
	collector.Histogram("payment_amount", 299.9, map[string]string{"channel": "alipay"})
	collector.Histogram("payment_duration_seconds", 0.5, map[string]string{"channel": "alipay"})

	// 支付失败
	collector.Counter("payment_failed_total", map[string]string{"channel": "wechat", "reason": "timeout"})
	collector.Counter("payment_failed_total", map[string]string{"channel": "wechat", "reason": "insufficient_balance"})

	// 待处理支付
	collector.Gauge("payment_pending", 5, map[string]string{"channel": "alipay"})
	collector.Gauge("payment_pending", 3, map[string]string{"channel": "wechat"})

	// 验证所有指标都被正确记录
	body := getMetrics(t, collector)
	assert.Contains(t, body, "payment_service_payment_success_total")
	assert.Contains(t, body, "payment_service_payment_failed_total")
	assert.Contains(t, body, "payment_service_payment_amount")
	assert.Contains(t, body, "payment_service_payment_duration_seconds")
	assert.Contains(t, body, "payment_service_payment_pending")
}

func getMetrics(t *testing.T, collector *PrometheusCollector) string {
	handler := collector.GetHandler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body, _ := io.ReadAll(rec.Body)
	return string(body)
}
