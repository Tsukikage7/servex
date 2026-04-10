# httpclient/ 统一 HTTP 客户端 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 围绕 `net/http.Client` 做 RoundTripper 层中间件链封装，复用现有 servex 组件（retry、circuitbreaker、tracing、metrics、discovery），提供 Response 包装和自动 JSON 序列化。

**Architecture:** Client 通过 Option 模式配置，内部在 `New()` 中按固定顺序（metrics → tracing → circuitbreaker → retry → logging → discovery → transport）构建 RoundTripper 中间件链。每个中间件适配器将现有 servex 组件包装为 `func(http.RoundTripper) http.RoundTripper` 接口。Response 封装 `*http.Response` 提供 JSON/Text/Bytes/CheckStatus 便捷方法。

**Tech Stack:** Go 标准库 net/http、encoding/json；servex 内部包 middleware/retry、middleware/circuitbreaker、observability/tracing、observability/metrics、discovery、logger、errors

---

## File Structure

| File | Responsibility |
|---|---|
| `httpclient/errors.go` | 包级 sentinel 错误 |
| `httpclient/response.go` | Response 包装，JSON/Text/Bytes/CheckStatus |
| `httpclient/response_test.go` | Response 测试 |
| `httpclient/middleware.go` | Middleware 类型 + Chain + 5 个内置适配器（retry/cb/tracing/metrics/logging） |
| `httpclient/middleware_test.go` | 中间件测试 |
| `httpclient/discovery.go` | 服务发现 RoundTripper（缓存 + 轮询） |
| `httpclient/discovery_test.go` | 服务发现测试 |
| `httpclient/options.go` | Option 类型 + 所有 With* 函数 |
| `httpclient/options_test.go` | Option 测试 |
| `httpclient/client.go` | Client struct、New、Do、Get/Post/Put/Delete |
| `httpclient/client_test.go` | Client 测试 |
| `httpclient/config.go` | Config struct + NewFromConfig 工厂 |
| `httpclient/config_test.go` | Config 测试 |

---

### Task 1: Response 包装与包级错误

**Files:**
- Create: `httpclient/errors.go`
- Create: `httpclient/response.go`
- Test: `httpclient/response_test.go`

- [ ] **Step 1: Write failing tests for Response**

```go
// httpclient/response_test.go
package httpclient

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	servexerrors "github.com/Tsukikage7/servex/errors"
	stderrors "errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newResponse(status int, body string) *Response {
	rec := httptest.NewRecorder()
	rec.WriteHeader(status)
	rec.WriteString(body)
	return &Response{Response: rec.Result()}
}

func TestResponse_JSON(t *testing.T) {
	resp := newResponse(200, `{"id":1,"name":"test"}`)
	var target struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	require.NoError(t, resp.JSON(&target))
	assert.Equal(t, 1, target.ID)
	assert.Equal(t, "test", target.Name)
}

func TestResponse_JSON_InvalidBody(t *testing.T) {
	resp := newResponse(200, `not json`)
	var target map[string]any
	assert.Error(t, resp.JSON(&target))
}

func TestResponse_Text(t *testing.T) {
	resp := newResponse(200, "hello world")
	text, err := resp.Text()
	require.NoError(t, err)
	assert.Equal(t, "hello world", text)
}

func TestResponse_Bytes(t *testing.T) {
	resp := newResponse(200, "binary data")
	b, err := resp.Bytes()
	require.NoError(t, err)
	assert.Equal(t, []byte("binary data"), b)
}

func TestResponse_CheckStatus_2xx(t *testing.T) {
	for _, code := range []int{200, 201, 204} {
		resp := newResponse(code, "")
		assert.NoError(t, resp.CheckStatus())
	}
}

func TestResponse_CheckStatus_4xx(t *testing.T) {
	resp := newResponse(404, "")
	err := resp.CheckStatus()
	require.Error(t, err)

	var e *servexerrors.Error
	require.True(t, stderrors.As(err, &e))
	assert.Equal(t, 404, e.Code)
	assert.Equal(t, 404, e.HTTP)
}

func TestResponse_CheckStatus_5xx(t *testing.T) {
	resp := newResponse(500, "")
	err := resp.CheckStatus()
	require.Error(t, err)

	var e *servexerrors.Error
	require.True(t, stderrors.As(err, &e))
	assert.Equal(t, 500, e.Code)
	assert.Equal(t, 500, e.HTTP)
}

func TestResponse_CheckStatus_ErrorsAs(t *testing.T) {
	resp := newResponse(403, "")
	err := resp.CheckStatus()
	var e *servexerrors.Error
	require.True(t, stderrors.As(err, &e))
	assert.Equal(t, "http.403", e.Key)
	assert.Contains(t, e.Message, "403")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/tsukikage/workspace/work/servex && go test ./httpclient/ -v -run TestResponse`
Expected: FAIL — package does not exist

- [ ] **Step 3: Implement errors.go and response.go**

```go
// httpclient/errors.go
package httpclient

import "errors"

var (
	ErrRequestFailed   = errors.New("httpclient: 请求创建失败")
	ErrDiscoveryFailed = errors.New("httpclient: 服务发现失败")
	ErrServiceNotFound = errors.New("httpclient: 未找到服务实例")
	ErrNoAddresses     = errors.New("httpclient: 无可用地址")
	ErrMarshalBody     = errors.New("httpclient: 请求体序列化失败")
)
```

```go
// httpclient/response.go
package httpclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Tsukikage7/servex/errors"
)

// Response HTTP 响应包装.
type Response struct {
	*http.Response
}

// JSON 反序列化 JSON body 到目标.
func (r *Response) JSON(v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// Text 读取文本.
func (r *Response) Text() (string, error) {
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Bytes 读取字节.
func (r *Response) Bytes() ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}

// CheckStatus 非 2xx 时返回 *errors.Error.
func (r *Response) CheckStatus() error {
	if r.StatusCode >= 200 && r.StatusCode < 300 {
		return nil
	}
	return errors.New(
		r.StatusCode,
		fmt.Sprintf("http.%d", r.StatusCode),
		fmt.Sprintf("HTTP %d: %s", r.StatusCode, http.StatusText(r.StatusCode)),
	).WithHTTP(r.StatusCode)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/tsukikage/workspace/work/servex && go test ./httpclient/ -v -run TestResponse`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add httpclient/errors.go httpclient/response.go httpclient/response_test.go
git commit -m "feat(httpclient): Response 包装与包级错误"
```

---

### Task 2: RoundTripper 中间件适配器

**Files:**
- Create: `httpclient/middleware.go`
- Test: `httpclient/middleware_test.go`

- [ ] **Step 1: Write failing tests for middleware adapters**

```go
// httpclient/middleware_test.go
package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/middleware/circuitbreaker"
	"github.com/Tsukikage7/servex/middleware/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTransport 可配置的 RoundTripper mock.
type mockTransport struct {
	responses []*http.Response
	errors    []error
	calls     atomic.Int32
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	i := int(m.calls.Add(1)) - 1
	if i < len(m.errors) && m.errors[i] != nil {
		return nil, m.errors[i]
	}
	if i < len(m.responses) {
		return m.responses[i], nil
	}
	// 默认返回 200
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("ok")),
		Header:     make(http.Header),
	}, nil
}

func resp(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}
}

func TestRetryMiddleware_NoRetryOnSuccess(t *testing.T) {
	mt := &mockTransport{responses: []*http.Response{resp(200)}}
	rt := RetryMiddleware(&retry.Config{MaxAttempts: 3, Delay: time.Millisecond, Backoff: retry.FixedBackoff})(mt)

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	r, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, 200, r.StatusCode)
	assert.Equal(t, int32(1), mt.calls.Load())
}

func TestRetryMiddleware_RetriesOn5xx(t *testing.T) {
	mt := &mockTransport{responses: []*http.Response{resp(503), resp(503), resp(200)}}
	rt := RetryMiddleware(&retry.Config{MaxAttempts: 3, Delay: time.Millisecond, Backoff: retry.FixedBackoff})(mt)

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	r, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, 200, r.StatusCode)
	assert.Equal(t, int32(3), mt.calls.Load())
}

func TestRetryMiddleware_ExhaustsRetries(t *testing.T) {
	mt := &mockTransport{responses: []*http.Response{resp(500), resp(500), resp(500)}}
	rt := RetryMiddleware(&retry.Config{MaxAttempts: 3, Delay: time.Millisecond, Backoff: retry.FixedBackoff})(mt)

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	r, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, 500, r.StatusCode)
	assert.Equal(t, int32(3), mt.calls.Load())
}

func TestRetryMiddleware_ContextCancellation(t *testing.T) {
	mt := &mockTransport{responses: []*http.Response{resp(503), resp(503), resp(503)}}
	rt := RetryMiddleware(&retry.Config{MaxAttempts: 5, Delay: 100 * time.Millisecond, Backoff: retry.FixedBackoff})(mt)

	ctx, cancel := context.WithCancel(context.Background())
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
	mt := &mockTransport{}
	mt.responses = []*http.Response{resp(503), resp(200)}
	original := http.RoundTripper(mt)
	// 包装一层来捕获 body
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
	mt := &mockTransport{responses: []*http.Response{resp(200)}}
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
	mt := &mockTransport{}
	// 所有请求返回 500
	for i := 0; i < 10; i++ {
		mt.responses = append(mt.responses, resp(500))
	}
	rt := CircuitBreakerMiddleware(cb)(mt)

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	// 触发足够多失败
	for i := 0; i < 3; i++ {
		rt.RoundTrip(req)
	}
	// 熔断器应该打开
	assert.Equal(t, circuitbreaker.StateOpen, cb.State())
}

func TestTracingMiddleware_InjectsHeaders(t *testing.T) {
	var capturedHeaders http.Header
	mt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		capturedHeaders = req.Header.Clone()
		return resp(200), nil
	})
	rt := TracingMiddleware("test-service")(mt)

	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	_, err := rt.RoundTrip(req)
	require.NoError(t, err)
	// tracing.InjectHTTPHeaders 会注入 traceparent（如果有 span 的话）
	// 至少不应该 panic
}

// mockCollector 简单 metrics mock.
type mockCollector struct {
	method     string
	path       string
	statusCode string
	duration   time.Duration
}

func (m *mockCollector) RecordHTTPRequest(method, path, statusCode string, duration time.Duration, reqSize, respSize float64) {
	m.method = method
	m.path = path
	m.statusCode = statusCode
	m.duration = duration
}
func (m *mockCollector) RecordGRPCRequest(string, string, string, time.Duration)     {}
func (m *mockCollector) RecordPanic(string, string, string)                           {}
func (m *mockCollector) UpdateGoroutineCount(int)                                     {}
func (m *mockCollector) UpdateMemoryUsage(int64)                                      {}
func (m *mockCollector) IncrementCounter(string, map[string]string)                   {}
func (m *mockCollector) ObserveHistogram(string, float64, map[string]string)          {}
func (m *mockCollector) SetGauge(string, float64, map[string]string)                  {}
func (m *mockCollector) GetHandler() http.Handler                                     { return nil }
func (m *mockCollector) GetPath() string                                              { return "" }

func TestMetricsMiddleware_RecordsRequest(t *testing.T) {
	mc := &mockCollector{}
	mt := &mockTransport{responses: []*http.Response{resp(200)}}
	rt := MetricsMiddleware(mc)(mt)

	req, _ := http.NewRequest("GET", "http://example.com/api/users", nil)
	_, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, "GET", mc.method)
	assert.Equal(t, "/api/users", mc.path)
	assert.Equal(t, "200", mc.statusCode)
	assert.True(t, mc.duration > 0)
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
	mt := &mockTransport{responses: []*http.Response{resp(200)}}
	rt := Chain(makeMW("A"), makeMW("B"), makeMW("C"))(mt)

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	rt.RoundTrip(req)

	assert.Equal(t, []string{"A:before", "B:before", "C:before", "C:after", "B:after", "A:after"}, order)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/tsukikage/workspace/work/servex && go test ./httpclient/ -v -run "TestRetry|TestCircuitBreaker|TestTracing|TestMetrics|TestChain"`
Expected: FAIL — types/functions undefined

- [ ] **Step 3: Implement middleware.go**

```go
// httpclient/middleware.go
package httpclient

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Tsukikage7/servex/logger"
	"github.com/Tsukikage7/servex/middleware/circuitbreaker"
	"github.com/Tsukikage7/servex/middleware/retry"
	"github.com/Tsukikage7/servex/observability/metrics"
	"github.com/Tsukikage7/servex/observability/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Middleware RoundTripper 中间件.
type Middleware func(http.RoundTripper) http.RoundTripper

// roundTripperFunc 函数适配器.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// Chain 按顺序组合中间件，outer 最先执行.
func Chain(outer Middleware, others ...Middleware) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		for i := len(others) - 1; i >= 0; i-- {
			next = others[i](next)
		}
		return outer(next)
	}
}

// RetryMiddleware 重试中间件.
func RetryMiddleware(cfg *retry.Config) Middleware {
	if cfg == nil {
		cfg = retry.DefaultConfig()
	}
	if cfg.Backoff == nil {
		cfg.Backoff = retry.FixedBackoff
	}
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			var bodyBytes []byte
			if req.Body != nil {
				var err error
				bodyBytes, err = io.ReadAll(req.Body)
				if err != nil {
					return nil, err
				}
				req.Body.Close()
			}

			var resp *http.Response
			var err error
			for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
				select {
				case <-req.Context().Done():
					return nil, req.Context().Err()
				default:
				}
				if bodyBytes != nil {
					req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				}
				resp, err = next.RoundTrip(req)
				if !retry.DefaultHTTPRetryable(resp, err) {
					return resp, err
				}
				if resp != nil {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
				if attempt < cfg.MaxAttempts-1 {
					wait := cfg.Backoff(attempt, cfg.Delay)
					select {
					case <-time.After(wait):
					case <-req.Context().Done():
						return nil, req.Context().Err()
					}
				}
			}
			return resp, err
		})
	}
}

// CircuitBreakerMiddleware 熔断器中间件.
func CircuitBreakerMiddleware(cb circuitbreaker.CircuitBreaker) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			var resp *http.Response
			err := cb.Execute(req.Context(), func() error {
				var e error
				resp, e = next.RoundTrip(req)
				if e != nil {
					return e
				}
				if resp.StatusCode >= 500 {
					return fmt.Errorf("server error: %d", resp.StatusCode)
				}
				return nil
			})
			if err != nil && resp != nil {
				// 5xx 情况：有响应但也有错误，返回响应
				return resp, nil
			}
			return resp, err
		})
	}
}

// TracingMiddleware 链路追踪中间件.
func TracingMiddleware(tracerName string) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			ctx, span := tracing.StartSpan(req.Context(), tracerName,
				req.Method+" "+req.URL.Path,
				trace.WithSpanKind(trace.SpanKindClient))
			defer span.End()

			req = req.WithContext(ctx)
			tracing.InjectHTTPHeaders(ctx, req)

			resp, err := next.RoundTrip(req)
			if err != nil {
				tracing.SetSpanError(ctx, err)
				return nil, err
			}
			tracing.SetSpanAttributes(ctx,
				attribute.Int("http.response.status_code", resp.StatusCode))
			return resp, nil
		})
	}
}

// MetricsMiddleware 请求指标中间件.
func MetricsMiddleware(collector metrics.Collector) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			start := time.Now()
			resp, err := next.RoundTrip(req)
			duration := time.Since(start)

			statusCode := "0"
			if resp != nil {
				statusCode = strconv.Itoa(resp.StatusCode)
			}
			collector.RecordHTTPRequest(req.Method, req.URL.Path, statusCode,
				duration, float64(req.ContentLength), 0)
			return resp, err
		})
	}
}

// LoggingMiddleware 请求日志中间件.
func LoggingMiddleware(log logger.Logger) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			start := time.Now()
			resp, err := next.RoundTrip(req)
			elapsed := time.Since(start)

			if err != nil {
				log.With(
					logger.String("method", req.Method),
					logger.String("url", req.URL.String()),
					logger.Duration("elapsed", elapsed),
					logger.Err(err),
				).Error("httpclient 请求失败")
				return nil, err
			}

			log.With(
				logger.String("method", req.Method),
				logger.String("url", req.URL.String()),
				logger.Int("status", resp.StatusCode),
				logger.Duration("elapsed", elapsed),
			).Debug("httpclient 请求完成")

			return resp, nil
		})
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/tsukikage/workspace/work/servex && go test ./httpclient/ -v -run "TestRetry|TestCircuitBreaker|TestTracing|TestMetrics|TestChain"`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add httpclient/middleware.go httpclient/middleware_test.go
git commit -m "feat(httpclient): RoundTripper 中间件适配器"
```

---

### Task 3: 服务发现 RoundTripper

**Files:**
- Create: `httpclient/discovery.go`
- Test: `httpclient/discovery_test.go`

- [ ] **Step 1: Write failing tests**

```go
// httpclient/discovery_test.go
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
)

type mockDiscovery struct {
	addrs []string
	err   error
	calls atomic.Int32
}

func (m *mockDiscovery) Register(ctx context.Context, serviceName, address string) (string, error) {
	return "", nil
}
func (m *mockDiscovery) RegisterWithProtocol(ctx context.Context, serviceName, address, protocol string) (string, error) {
	return "", nil
}
func (m *mockDiscovery) RegisterWithHealthEndpoint(ctx context.Context, serviceName, address, protocol string, healthEndpoint any) (string, error) {
	return "", nil
}
func (m *mockDiscovery) Unregister(ctx context.Context, serviceID string) error { return nil }
func (m *mockDiscovery) Close() error                                           { return nil }
func (m *mockDiscovery) Discover(ctx context.Context, serviceName string) ([]string, error) {
	m.calls.Add(1)
	return m.addrs, m.err
}

func TestDiscoveryMiddleware_ResolvesAddress(t *testing.T) {
	disc := &mockDiscovery{addrs: []string{"localhost:9090"}}
	var capturedHost string
	inner := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		capturedHost = req.URL.Host
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     make(http.Header),
		}, nil
	})
	rt := DiscoveryMiddleware(disc, "my-service")(inner)

	req, _ := http.NewRequest("GET", "http://placeholder/api", nil)
	_, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, "localhost:9090", capturedHost)
}

func TestDiscoveryMiddleware_RoundRobin(t *testing.T) {
	disc := &mockDiscovery{addrs: []string{"host1:80", "host2:80", "host3:80"}}
	var hosts []string
	inner := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		hosts = append(hosts, req.URL.Host)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     make(http.Header),
		}, nil
	})
	rt := DiscoveryMiddleware(disc, "my-service")(inner)

	for i := 0; i < 6; i++ {
		req, _ := http.NewRequest("GET", "http://placeholder/api", nil)
		rt.RoundTrip(req)
	}
	assert.Equal(t, []string{"host1:80", "host2:80", "host3:80", "host1:80", "host2:80", "host3:80"}, hosts)
}

func TestDiscoveryMiddleware_CachesResults(t *testing.T) {
	disc := &mockDiscovery{addrs: []string{"host1:80"}}
	inner := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     make(http.Header),
		}, nil
	})
	rt := DiscoveryMiddleware(disc, "my-service")(inner)

	req, _ := http.NewRequest("GET", "http://placeholder/api", nil)
	rt.RoundTrip(req)
	rt.RoundTrip(req)
	rt.RoundTrip(req)

	// 缓存期间只调用一次 Discover
	assert.Equal(t, int32(1), disc.calls.Load())
}

func TestDiscoveryMiddleware_DiscoveryError_NoCache(t *testing.T) {
	disc := &mockDiscovery{err: assert.AnError}
	inner := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return resp(200), nil
	})
	rt := DiscoveryMiddleware(disc, "my-service")(inner)

	req, _ := http.NewRequest("GET", "http://placeholder/api", nil)
	_, err := rt.RoundTrip(req)
	assert.ErrorIs(t, err, ErrDiscoveryFailed)
}

func TestDiscoveryMiddleware_EmptyAddresses(t *testing.T) {
	disc := &mockDiscovery{addrs: []string{}}
	inner := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return resp(200), nil
	})
	rt := DiscoveryMiddleware(disc, "my-service")(inner)

	req, _ := http.NewRequest("GET", "http://placeholder/api", nil)
	_, err := rt.RoundTrip(req)
	assert.ErrorIs(t, err, ErrServiceNotFound)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/tsukikage/workspace/work/servex && go test ./httpclient/ -v -run TestDiscovery`
Expected: FAIL — DiscoveryMiddleware undefined

- [ ] **Step 3: Implement discovery.go**

```go
// httpclient/discovery.go
package httpclient

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Tsukikage7/servex/discovery"
)

const defaultCacheTTL = 10 * time.Second

type discoveryTransport struct {
	next        http.RoundTripper
	disc        discovery.Discovery
	serviceName string
	mu          sync.RWMutex
	addrs       []string
	lastRefresh time.Time
	index       atomic.Uint64
}

// DiscoveryMiddleware 服务发现中间件，缓存 + 轮询.
func DiscoveryMiddleware(disc discovery.Discovery, serviceName string) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return &discoveryTransport{
			next:        next,
			disc:        disc,
			serviceName: serviceName,
		}
	}
}

func (d *discoveryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	addr, err := d.pick(req)
	if err != nil {
		return nil, err
	}
	req = req.Clone(req.Context())
	req.URL.Host = addr
	return d.next.RoundTrip(req)
}

func (d *discoveryTransport) pick(req *http.Request) (string, error) {
	d.mu.RLock()
	addrs := d.addrs
	lastRefresh := d.lastRefresh
	d.mu.RUnlock()

	if len(addrs) == 0 || time.Since(lastRefresh) > defaultCacheTTL {
		newAddrs, err := d.disc.Discover(req.Context(), d.serviceName)
		if err != nil {
			if len(addrs) > 0 {
				// 使用缓存的地址
			} else {
				return "", fmt.Errorf("%w: %v", ErrDiscoveryFailed, err)
			}
		} else if len(newAddrs) == 0 {
			if len(addrs) == 0 {
				return "", fmt.Errorf("%w: %s", ErrServiceNotFound, d.serviceName)
			}
		} else {
			d.mu.Lock()
			d.addrs = newAddrs
			d.lastRefresh = time.Now()
			addrs = newAddrs
			d.mu.Unlock()
		}
	}

	if len(addrs) == 0 {
		return "", fmt.Errorf("%w: %s", ErrNoAddresses, d.serviceName)
	}

	idx := d.index.Add(1) - 1
	return addrs[int(idx)%len(addrs)], nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/tsukikage/workspace/work/servex && go test ./httpclient/ -v -run TestDiscovery`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add httpclient/discovery.go httpclient/discovery_test.go
git commit -m "feat(httpclient): 服务发现 RoundTripper 中间件"
```

---

### Task 4: Option 函数

**Files:**
- Create: `httpclient/options.go`
- Test: `httpclient/options_test.go`

- [ ] **Step 1: Write failing tests**

```go
// httpclient/options_test.go
package httpclient

import (
	"net/http"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/middleware/circuitbreaker"
	"github.com/Tsukikage7/servex/middleware/retry"
	"github.com/stretchr/testify/assert"
)

func TestDefaultOptions(t *testing.T) {
	o := defaultOptions()
	assert.Equal(t, 30*time.Second, o.timeout)
	assert.NotNil(t, o.headers)
	assert.Empty(t, o.baseURL)
	assert.Nil(t, o.logger)
}

func TestWithBaseURL(t *testing.T) {
	o := defaultOptions()
	WithBaseURL("http://example.com")(o)
	assert.Equal(t, "http://example.com", o.baseURL)
}

func TestWithTimeout(t *testing.T) {
	o := defaultOptions()
	WithTimeout(5 * time.Second)(o)
	assert.Equal(t, 5*time.Second, o.timeout)
}

func TestWithHeader(t *testing.T) {
	o := defaultOptions()
	WithHeader("X-Service", "test")(o)
	assert.Equal(t, "test", o.headers.Get("X-Service"))
}

func TestWithHeaders(t *testing.T) {
	o := defaultOptions()
	WithHeaders(map[string]string{"X-A": "1", "X-B": "2"})(o)
	assert.Equal(t, "1", o.headers.Get("X-A"))
	assert.Equal(t, "2", o.headers.Get("X-B"))
}

func TestWithTransport(t *testing.T) {
	o := defaultOptions()
	tr := &http.Transport{}
	WithTransport(tr)(o)
	assert.Equal(t, tr, o.transport)
}

func TestWithRetry(t *testing.T) {
	o := defaultOptions()
	cfg := &retry.Config{MaxAttempts: 3}
	WithRetry(cfg)(o)
	assert.Equal(t, cfg, o.retryCfg)
}

func TestWithCircuitBreaker(t *testing.T) {
	o := defaultOptions()
	cb := circuitbreaker.New()
	WithCircuitBreaker(cb)(o)
	assert.Equal(t, cb, o.circuitBreaker)
}

func TestWithTracing(t *testing.T) {
	o := defaultOptions()
	WithTracing("my-service")(o)
	assert.Equal(t, "my-service", o.tracerName)
}

func TestWithMetrics(t *testing.T) {
	o := defaultOptions()
	mc := &mockCollector{}
	WithMetrics(mc)(o)
	assert.Equal(t, mc, o.metricsCollector)
}

func TestWithDiscovery(t *testing.T) {
	o := defaultOptions()
	disc := &mockDiscovery{addrs: []string{"host:80"}}
	WithDiscovery(disc, "my-service")(o)
	assert.Equal(t, disc, o.disc)
	assert.Equal(t, "my-service", o.discServiceName)
}

func TestWithMiddleware(t *testing.T) {
	o := defaultOptions()
	mw := func(next http.RoundTripper) http.RoundTripper { return next }
	WithMiddleware(mw)(o)
	assert.Len(t, o.middlewares, 1)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/tsukikage/workspace/work/servex && go test ./httpclient/ -v -run "TestDefault|TestWith"`
Expected: FAIL — types/functions undefined

- [ ] **Step 3: Implement options.go**

```go
// httpclient/options.go
package httpclient

import (
	"net/http"
	"time"

	"github.com/Tsukikage7/servex/discovery"
	"github.com/Tsukikage7/servex/logger"
	"github.com/Tsukikage7/servex/middleware/circuitbreaker"
	"github.com/Tsukikage7/servex/middleware/retry"
	"github.com/Tsukikage7/servex/observability/metrics"
)

// Option 客户端配置选项.
type Option func(*options)

type options struct {
	baseURL          string
	timeout          time.Duration
	headers          http.Header
	logger           logger.Logger
	transport        http.RoundTripper
	retryCfg         *retry.Config
	circuitBreaker   circuitbreaker.CircuitBreaker
	tracerName       string
	metricsCollector metrics.Collector
	disc             discovery.Discovery
	discServiceName  string
	middlewares      []Middleware
}

func defaultOptions() *options {
	return &options{
		timeout: 30 * time.Second,
		headers: make(http.Header),
	}
}

// WithBaseURL 设置基础 URL.
func WithBaseURL(url string) Option { return func(o *options) { o.baseURL = url } }

// WithTimeout 设置请求超时.
func WithTimeout(d time.Duration) Option { return func(o *options) { o.timeout = d } }

// WithLogger 设置日志记录器.
func WithLogger(l logger.Logger) Option { return func(o *options) { o.logger = l } }

// WithHeader 设置默认请求头.
func WithHeader(key, value string) Option {
	return func(o *options) { o.headers.Set(key, value) }
}

// WithHeaders 批量设置默认请求头.
func WithHeaders(h map[string]string) Option {
	return func(o *options) {
		for k, v := range h {
			o.headers.Set(k, v)
		}
	}
}

// WithTransport 设置自定义 Transport.
func WithTransport(rt http.RoundTripper) Option {
	return func(o *options) { o.transport = rt }
}

// WithRetry 启用重试.
func WithRetry(cfg *retry.Config) Option {
	return func(o *options) { o.retryCfg = cfg }
}

// WithCircuitBreaker 启用熔断器.
func WithCircuitBreaker(cb circuitbreaker.CircuitBreaker) Option {
	return func(o *options) { o.circuitBreaker = cb }
}

// WithTracing 启用链路追踪.
func WithTracing(tracerName string) Option {
	return func(o *options) { o.tracerName = tracerName }
}

// WithMetrics 启用请求指标.
func WithMetrics(c metrics.Collector) Option {
	return func(o *options) { o.metricsCollector = c }
}

// WithDiscovery 启用服务发现.
func WithDiscovery(d discovery.Discovery, serviceName string) Option {
	return func(o *options) { o.disc = d; o.discServiceName = serviceName }
}

// WithMiddleware 添加自定义中间件.
func WithMiddleware(mw ...Middleware) Option {
	return func(o *options) { o.middlewares = append(o.middlewares, mw...) }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/tsukikage/workspace/work/servex && go test ./httpclient/ -v -run "TestDefault|TestWith"`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add httpclient/options.go httpclient/options_test.go
git commit -m "feat(httpclient): Option 函数"
```

---

### Task 5: Client 核心与请求方法

**Files:**
- Create: `httpclient/client.go`
- Test: `httpclient/client_test.go`

- [ ] **Step 1: Write failing tests**

```go
// httpclient/client_test.go
package httpclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	stderrors "errors"

	servexerrors "github.com/Tsukikage7/servex/errors"
	"github.com/Tsukikage7/servex/middleware/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_DefaultOptions(t *testing.T) {
	c := New(WithBaseURL("http://example.com"))
	assert.NotNil(t, c)
	assert.Equal(t, "http://example.com", c.base)
}

func TestClient_Get(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/users/1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": 1, "name": "test"})
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	resp, err := c.Get(context.Background(), "/api/users/1")
	require.NoError(t, err)

	var result struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	require.NoError(t, resp.JSON(&result))
	assert.Equal(t, 1, result.ID)
	assert.Equal(t, "test", result.Name)
}

func TestClient_Post_JSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "test", body["name"])

		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{"id": 1})
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	resp, err := c.Post(context.Background(), "/api/users", map[string]string{"name": "test"})
	require.NoError(t, err)
	assert.Equal(t, 201, resp.StatusCode)
}

func TestClient_Put(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	resp, err := c.Put(context.Background(), "/api/users/1", map[string]string{"name": "updated"})
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestClient_Delete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(204)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	resp, err := c.Delete(context.Background(), "/api/users/1")
	require.NoError(t, err)
	assert.Equal(t, 204, resp.StatusCode)
}

func TestClient_Do_WithQueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "1", r.URL.Query().Get("page"))
		assert.Equal(t, "10", r.URL.Query().Get("size"))
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	resp, err := c.Do(context.Background(), &Request{
		Method: http.MethodGet,
		Path:   "/api/users",
		Query:  map[string]string{"page": "1", "size": "10"},
	})
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestClient_Do_WithPerRequestHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "req-123", r.Header.Get("X-Request-ID"))
		assert.Equal(t, "order-service", r.Header.Get("X-Service"))
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL), WithHeader("X-Service", "order-service"))
	resp, err := c.Do(context.Background(), &Request{
		Method:  http.MethodGet,
		Path:    "/api",
		Headers: map[string]string{"X-Request-ID": "req-123"},
	})
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestClient_Do_DefaultHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "order-service", r.Header.Get("X-Service"))
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL), WithHeader("X-Service", "order-service"))
	resp, err := c.Get(context.Background(), "/api")
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestClient_Do_MarshalError(t *testing.T) {
	c := New(WithBaseURL("http://example.com"))
	_, err := c.Post(context.Background(), "/api", make(chan int))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMarshalBody)
}

func TestClient_ResponseCheckStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	resp, err := c.Get(context.Background(), "/not-found")
	require.NoError(t, err)

	err = resp.CheckStatus()
	require.Error(t, err)

	var e *servexerrors.Error
	require.True(t, stderrors.As(err, &e))
	assert.Equal(t, 404, e.Code)
}

func TestClient_IntegrationWithRetry(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
		io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	c := New(
		WithBaseURL(srv.URL),
		WithRetry(&retry.Config{
			MaxAttempts: 3,
			Delay:       time.Millisecond,
			Backoff:     retry.FixedBackoff,
		}),
	)
	resp, err := c.Get(context.Background(), "/api")
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 3, attempts)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/tsukikage/workspace/work/servex && go test ./httpclient/ -v -run "TestNew|TestClient"`
Expected: FAIL — Client, New, Request undefined

- [ ] **Step 3: Implement client.go**

```go
// httpclient/client.go
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Tsukikage7/servex/logger"
)

// Client 封装 http.Client，支持中间件链.
type Client struct {
	http    *http.Client
	base    string
	headers http.Header
	logger  logger.Logger
}

// Request 请求描述.
type Request struct {
	Method  string
	Path    string
	Body    any
	Headers map[string]string
	Query   map[string]string
}

// New 创建客户端.
func New(opts ...Option) *Client {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	var rt http.RoundTripper = o.transport
	if rt == nil {
		rt = http.DefaultTransport
	}

	// 从内到外构建中间件链：
	// discovery（最内层）→ logging → retry → circuitbreaker → tracing → metrics（最外层）
	if o.disc != nil {
		rt = DiscoveryMiddleware(o.disc, o.discServiceName)(rt)
	}
	if o.logger != nil {
		rt = LoggingMiddleware(o.logger)(rt)
	}
	if o.retryCfg != nil {
		rt = RetryMiddleware(o.retryCfg)(rt)
	}
	if o.circuitBreaker != nil {
		rt = CircuitBreakerMiddleware(o.circuitBreaker)(rt)
	}
	if o.tracerName != "" {
		rt = TracingMiddleware(o.tracerName)(rt)
	}
	if o.metricsCollector != nil {
		rt = MetricsMiddleware(o.metricsCollector)(rt)
	}

	// 自定义中间件
	for i := len(o.middlewares) - 1; i >= 0; i-- {
		rt = o.middlewares[i](rt)
	}

	return &Client{
		http: &http.Client{
			Timeout:   o.timeout,
			Transport: rt,
		},
		base:    o.baseURL,
		headers: o.headers,
		logger:  o.logger,
	}
}

// Do 发送请求.
func (c *Client) Do(ctx context.Context, r *Request) (*Response, error) {
	var bodyReader *bytes.Reader
	if r.Body != nil {
		b, err := json.Marshal(r.Body)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrMarshalBody, err)
		}
		bodyReader = bytes.NewReader(b)
	}

	url := c.base + r.Path

	var req *http.Request
	var err error
	if bodyReader != nil {
		req, err = http.NewRequestWithContext(ctx, r.Method, url, bodyReader)
	} else {
		req, err = http.NewRequestWithContext(ctx, r.Method, url, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}

	if r.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for key, vals := range c.headers {
		for _, v := range vals {
			req.Header.Set(key, v)
		}
	}

	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}

	if len(r.Query) > 0 {
		q := req.URL.Query()
		for k, v := range r.Query {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	return &Response{Response: resp}, nil
}

// Get 发送 GET 请求.
func (c *Client) Get(ctx context.Context, path string) (*Response, error) {
	return c.Do(ctx, &Request{Method: http.MethodGet, Path: path})
}

// Post 发送 POST 请求.
func (c *Client) Post(ctx context.Context, path string, body any) (*Response, error) {
	return c.Do(ctx, &Request{Method: http.MethodPost, Path: path, Body: body})
}

// Put 发送 PUT 请求.
func (c *Client) Put(ctx context.Context, path string, body any) (*Response, error) {
	return c.Do(ctx, &Request{Method: http.MethodPut, Path: path, Body: body})
}

// Delete 发送 DELETE 请求.
func (c *Client) Delete(ctx context.Context, path string) (*Response, error) {
	return c.Do(ctx, &Request{Method: http.MethodDelete, Path: path})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/tsukikage/workspace/work/servex && go test ./httpclient/ -v -run "TestNew|TestClient"`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add httpclient/client.go httpclient/client_test.go
git commit -m "feat(httpclient): Client 核心与 Get/Post/Put/Delete"
```

---

### Task 6: Config 驱动工厂

**Files:**
- Create: `httpclient/config.go`
- Test: `httpclient/config_test.go`

- [ ] **Step 1: Write failing tests**

```go
// httpclient/config_test.go
package httpclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFromConfig_MinimalConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	c := NewFromConfig(&Config{
		BaseURL: srv.URL,
		Timeout: 5 * time.Second,
	})
	require.NotNil(t, c)

	resp, err := c.Get(context.Background(), "/api")
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestNewFromConfig_WithRetry(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	c := NewFromConfig(&Config{
		BaseURL:    srv.URL,
		Timeout:    5 * time.Second,
		MaxRetries: 3,
		RetryDelay: time.Millisecond,
	})

	resp, err := c.Get(context.Background(), "/api")
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 3, attempts)
}

func TestNewFromConfig_WithAdditionalOpts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-service", r.Header.Get("X-Service"))
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := NewFromConfig(
		&Config{BaseURL: srv.URL},
		WithHeader("X-Service", "test-service"),
	)

	resp, err := c.Get(context.Background(), "/api")
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestNewFromConfig_ZeroValues(t *testing.T) {
	c := NewFromConfig(&Config{})
	require.NotNil(t, c)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/tsukikage/workspace/work/servex && go test ./httpclient/ -v -run "TestNewFromConfig|TestConfig"`
Expected: FAIL — Config, NewFromConfig undefined

- [ ] **Step 3: Implement config.go**

```go
// httpclient/config.go
package httpclient

import (
	"time"

	"github.com/Tsukikage7/servex/middleware/circuitbreaker"
	"github.com/Tsukikage7/servex/middleware/retry"
)

// Config 配置驱动的客户端创建.
type Config struct {
	BaseURL        string        `json:"base_url" yaml:"base_url" mapstructure:"base_url"`
	Timeout        time.Duration `json:"timeout" yaml:"timeout" mapstructure:"timeout"`
	MaxRetries     int           `json:"max_retries" yaml:"max_retries" mapstructure:"max_retries"`
	RetryDelay     time.Duration `json:"retry_delay" yaml:"retry_delay" mapstructure:"retry_delay"`
	CircuitBreaker bool          `json:"circuit_breaker" yaml:"circuit_breaker" mapstructure:"circuit_breaker"`
	Tracing        bool          `json:"tracing" yaml:"tracing" mapstructure:"tracing"`
}

// NewFromConfig 从配置创建客户端.
func NewFromConfig(cfg *Config, additionalOpts ...Option) *Client {
	var opts []Option

	if cfg.BaseURL != "" {
		opts = append(opts, WithBaseURL(cfg.BaseURL))
	}
	if cfg.Timeout > 0 {
		opts = append(opts, WithTimeout(cfg.Timeout))
	}
	if cfg.MaxRetries > 0 {
		opts = append(opts, WithRetry(&retry.Config{
			MaxAttempts: cfg.MaxRetries,
			Delay:       cfg.RetryDelay,
			Backoff:     retry.FixedBackoff,
			Retryable:   retry.AlwaysRetry,
		}))
	}
	if cfg.CircuitBreaker {
		opts = append(opts, WithCircuitBreaker(circuitbreaker.New()))
	}
	if cfg.Tracing {
		opts = append(opts, WithTracing("httpclient"))
	}

	opts = append(opts, additionalOpts...)
	return New(opts...)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/tsukikage/workspace/work/servex && go test ./httpclient/ -v -run "TestNewFromConfig|TestConfig"`
Expected: PASS

- [ ] **Step 5: Run all httpclient tests**

Run: `cd /Users/tsukikage/workspace/work/servex && go test ./httpclient/ -v -count=1`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add httpclient/config.go httpclient/config_test.go
git commit -m "feat(httpclient): Config 驱动工厂"
```
