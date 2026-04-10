package httpclient

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/Tsukikage7/servex/middleware/circuitbreaker"
	"github.com/Tsukikage7/servex/middleware/retry"
	"github.com/Tsukikage7/servex/observability/logger"
	"github.com/Tsukikage7/servex/observability/metrics"
	"github.com/Tsukikage7/servex/observability/tracing"
)

// Middleware 是 http.RoundTripper 的中间件类型.
type Middleware func(http.RoundTripper) http.RoundTripper

// Chain 将多个中间件组合成一个，outer 最先执行.
func Chain(outer Middleware, others ...Middleware) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		for i := len(others) - 1; i >= 0; i-- {
			next = others[i](next)
		}
		return outer(next)
	}
}

// LoggingMiddleware 记录请求耗时和响应状态的中间件.
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
				).Error("[HTTP] 请求失败")
				return nil, err
			}

			log.With(
				logger.String("method", req.Method),
				logger.String("url", req.URL.String()),
				logger.Int("status", resp.StatusCode),
				logger.Duration("elapsed", elapsed),
			).Debug("[HTTP] 请求完成")

			return resp, nil
		})
	}
}

// roundTripperFunc 将函数适配为 http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
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
//
// 5xx 响应视为失败触发熔断计数，但仍返回原始响应给调用方（而非返回错误），
// 以便调用方可以根据响应体做进一步处理.
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
					// 5xx 作为熔断计数的失败信号，但不丢弃响应
					return fmt.Errorf("server error: %d", resp.StatusCode)
				}
				return nil
			})
			// 如果有响应（包括 5xx），优先返回响应而非熔断错误
			if resp != nil {
				return resp, nil
			}
			return nil, err
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
