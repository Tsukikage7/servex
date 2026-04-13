package metrics

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/Tsukikage7/servex/v2/endpoint"
)

// HTTPMiddleware 返回 HTTP 指标采集中间件.
// 使用示例:
//
//	collector, _ := metrics.New(cfg)
//	handler := metrics.HTTPMiddleware(collector)(mux)
//	http.ListenAndServe(":8080", handler)
func HTTPMiddleware(collector *PrometheusCollector) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// 包装 ResponseWriter 捕获状态码和响应大小
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// 执行下一个处理器
			next.ServeHTTP(rw, r)

			// 记录指标
			collector.RecordHTTPRequest(
				r.Method,
				r.URL.Path,
				strconv.Itoa(rw.statusCode),
				time.Since(start),
				float64(r.ContentLength),
				float64(rw.size),
			)
		})
	}
}

// responseWriter 包装 http.ResponseWriter 以捕获状态码和响应大小.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

// Flush 实现 http.Flusher 接口，确保 SSE 等流式响应正常工作.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap 返回底层 ResponseWriter，支持 http.ResponseController 等标准库功能.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// EndpointMiddleware 返回 Endpoint 指标采集中间件.
// 使用示例:
//
//	collector, _ := metrics.New(cfg)
//	endpoint = metrics.EndpointMiddleware(collector, "my-service", "GetUser")(endpoint)
func EndpointMiddleware(collector *PrometheusCollector, service, method string) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (response any, err error) {
			start := time.Now()

			response, err = next(ctx, request)

			// 记录指标
			statusCode := "OK"
			if err != nil {
				statusCode = "ERROR"
			}
			collector.RecordGRPCRequest(method, service, statusCode, time.Since(start))

			return response, err
		}
	}
}

// EndpointInstrumenter 提供可配置的 Endpoint 指标采集.
type EndpointInstrumenter struct {
	collector *PrometheusCollector
	service   string
}

// NewEndpointInstrumenter 创建 Endpoint 指标采集器.
func NewEndpointInstrumenter(collector *PrometheusCollector, service string) *EndpointInstrumenter {
	return &EndpointInstrumenter{
		collector: collector,
		service:   service,
	}
}

// Middleware 返回指定方法的指标中间件.
func (i *EndpointInstrumenter) Middleware(method string) endpoint.Middleware {
	return EndpointMiddleware(i.collector, i.service, method)
}
