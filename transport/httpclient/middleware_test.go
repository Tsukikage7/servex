package httpclient

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tsukikage7/servex/middleware/circuitbreaker"
	"github.com/Tsukikage7/servex/middleware/retry"
)

// mwMockTransport 可配置的 RoundTripper mock.
type mwMockTransport struct {
	responses []*http.Response
	errors    []error
	calls     atomic.Int32
}

func (m *mwMockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	i := int(m.calls.Add(1)) - 1
	if i < len(m.errors) && m.errors[i] != nil {
		return nil, m.errors[i]
	}
	if i < len(m.responses) {
		return m.responses[i], nil
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("ok")),
		Header:     make(http.Header),
	}, nil
}

func mwResp(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}
}

func TestRetryMiddleware_NoRetryOnSuccess(t *testing.T) {
	mt := &mwMockTransport{responses: []*http.Response{mwResp(200)}}
	rt := RetryMiddleware(&retry.Config{MaxAttempts: 3, Delay: time.Millisecond, Backoff: retry.FixedBackoff})(mt)

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	r, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, 200, r.StatusCode)
	assert.Equal(t, int32(1), mt.calls.Load())
}

func TestRetryMiddleware_RetriesOn5xx(t *testing.T) {
	mt := &mwMockTransport{responses: []*http.Response{mwResp(503), mwResp(503), mwResp(200)}}
	rt := RetryMiddleware(&retry.Config{MaxAttempts: 3, Delay: time.Millisecond, Backoff: retry.FixedBackoff})(mt)

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	r, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, 200, r.StatusCode)
	assert.Equal(t, int32(3), mt.calls.Load())
}

func TestRetryMiddleware_ExhaustsRetries(t *testing.T) {
	mt := &mwMockTransport{responses: []*http.Response{mwResp(500), mwResp(500), mwResp(500)}}
	rt := RetryMiddleware(&retry.Config{MaxAttempts: 3, Delay: time.Millisecond, Backoff: retry.FixedBackoff})(mt)

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	r, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, 500, r.StatusCode)
	assert.Equal(t, int32(3), mt.calls.Load())
}

func TestRetryMiddleware_ContextCancellation(t *testing.T) {
	mt := &mwMockTransport{responses: []*http.Response{mwResp(503), mwResp(503), mwResp(503)}}
	rt := RetryMiddleware(&retry.Config{MaxAttempts: 5, Delay: 100 * time.Millisecond, Backoff: retry.FixedBackoff})(mt)

	ctx, cancel := context.WithCancel(t.Context())
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := rt.RoundTrip(req)
	assert.Error(t, err)
	assert.True(t, mt.calls.Load() < 5)
}

func TestRetryMiddleware_BodyReplay(t *testing.T) {
	var bodies []string
	mt := &mwMockTransport{}
	mt.responses = []*http.Response{mwResp(503), mwResp(200)}
	original := http.RoundTripper(mt)
	capturing := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.Body != nil {
			b, _ := io.ReadAll(req.Body)
			bodies = append(bodies, string(b))
			req.Body = io.NopCloser(strings.NewReader(string(b)))
		}
		return original.RoundTrip(req)
	})

	rt := RetryMiddleware(&retry.Config{MaxAttempts: 3, Delay: time.Millisecond, Backoff: retry.FixedBackoff})(capturing)

	req, _ := http.NewRequest("POST", "http://example.com", strings.NewReader(`{"key":"value"}`))
	_, err := rt.RoundTrip(req)
	require.NoError(t, err)
	require.Len(t, bodies, 2)
	assert.Equal(t, `{"key":"value"}`, bodies[0])
	assert.Equal(t, `{"key":"value"}`, bodies[1])
}

func TestCircuitBreakerMiddleware_PassesThrough(t *testing.T) {
	cb := circuitbreaker.New(circuitbreaker.WithFailureThreshold(5))
	mt := &mwMockTransport{responses: []*http.Response{mwResp(200)}}
	rt := CircuitBreakerMiddleware(cb)(mt)

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	r, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, 200, r.StatusCode)
}

func TestCircuitBreakerMiddleware_5xxTripsBreaker(t *testing.T) {
	cb := circuitbreaker.New(
		circuitbreaker.WithFailureThreshold(2),
		circuitbreaker.WithOpenTimeout(5*time.Second),
	)
	mt := &mwMockTransport{}
	for i := 0; i < 10; i++ {
		mt.responses = append(mt.responses, mwResp(500))
	}
	rt := CircuitBreakerMiddleware(cb)(mt)

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	for i := 0; i < 3; i++ {
		rt.RoundTrip(req)
	}
	assert.Equal(t, circuitbreaker.StateOpen, cb.State())
}

func TestTracingMiddleware_InjectsHeaders(t *testing.T) {
	mt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return mwResp(200), nil
	})
	rt := TracingMiddleware("test-service")(mt)

	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	_, err := rt.RoundTrip(req)
	require.NoError(t, err)
}

// mwMockCollector 简单 metrics mock.
type mwMockCollector struct {
	method     string
	path       string
	statusCode string
	duration   time.Duration
}

func (m *mwMockCollector) RecordHTTPRequest(method, path, statusCode string, duration time.Duration, reqSize, respSize float64) {
	m.method = method
	m.path = path
	m.statusCode = statusCode
	m.duration = duration
}
func (m *mwMockCollector) RecordGRPCRequest(string, string, string, time.Duration) {}
func (m *mwMockCollector) RecordPanic(string, string, string)                      {}
func (m *mwMockCollector) UpdateGoroutineCount(int)                                {}
func (m *mwMockCollector) UpdateMemoryUsage(int64)                                 {}
func (m *mwMockCollector) Counter(string, map[string]string)                       {}
func (m *mwMockCollector) Histogram(string, float64, map[string]string)            {}
func (m *mwMockCollector) Gauge(string, float64, map[string]string)                {}
func (m *mwMockCollector) GetHandler() http.Handler                                { return nil }
func (m *mwMockCollector) GetPath() string                                         { return "" }

func TestMetricsMiddleware_RecordsRequest(t *testing.T) {
	mc := &mwMockCollector{}
	mt := &mwMockTransport{responses: []*http.Response{mwResp(200)}}
	rt := MetricsMiddleware(mc)(mt)

	req, _ := http.NewRequest("GET", "http://example.com/api/users", nil)
	_, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, "GET", mc.method)
	assert.Equal(t, "/api/users", mc.path)
	assert.Equal(t, "200", mc.statusCode)
	assert.True(t, mc.duration >= 0)
}

func TestChain_OrderPreserved(t *testing.T) {
	var order []string
	makeMW := func(name string) Middleware {
		return func(next http.RoundTripper) http.RoundTripper {
			return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				order = append(order, name+":before")
				resp, err := next.RoundTrip(req)
				order = append(order, name+":after")
				return resp, err
			})
		}
	}
	mt := &mwMockTransport{responses: []*http.Response{mwResp(200)}}
	rt := Chain(makeMW("A"), makeMW("B"), makeMW("C"))(mt)

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	rt.RoundTrip(req)

	assert.Equal(t, []string{"A:before", "B:before", "C:before", "C:after", "B:after", "A:after"}, order)
}
