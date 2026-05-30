package tracing

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/Tsukikage7/servex/v2/endpoint"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// TraceIDHeader 响应头中返回 traceId 的键名.
const TraceIDHeader = "X-Trace-Id"

// HTTPMiddleware 返回 HTTP 链路追踪中间件.
// 中间件会自动从请求头提取或生成 traceId，并通过响应头 X-Trace-Id 返回.
// traceId 同时作为请求的唯一标识requestId，可通过 TraceID(ctx) 获取.
// skipPaths 指定不产生 trace span 的路径如 /metrics、/healthz.
// 使用示例:
//
//	mux := http.NewServeMux()
//	mux.HandleFunc("/api/users", handleUsers)
//	handler := trace.HTTPMiddleware("my-service", "/metrics", "/healthz")(mux)
//	http.ListenAndServe(":8080", handler)
//	// 在处理器中获取 traceId
//	func handleUsers(w http.ResponseWriter, r *http.Request) {
//	    traceId := trace.TraceID(r.Context())
//	    // ...
//	}
func HTTPMiddleware(serviceName string, skipPaths ...string) func(http.Handler) http.Handler {
	skip := make(map[string]bool, len(skipPaths))
	for _, p := range skipPaths {
		skip[p] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skip[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// 从请求头提取上下文
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			tracer := otel.Tracer(serviceName)
			spanName := r.Method + " " + r.URL.Path

			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPRequestMethodKey.String(r.Method),
					semconv.URLFull(r.URL.String()),
					semconv.URLPath(r.URL.Path),
					semconv.ServerAddress(r.Host),
					semconv.UserAgentOriginal(r.UserAgent()),
					semconv.URLScheme(r.URL.Scheme),
				),
			)
			defer span.End()

			// 注入 trace 信息到 context供 middleware/trace 读取
			spanCtx := span.SpanContext()
			if spanCtx.HasTraceID() {
				ctx = logger.ContextWithTraceID(ctx, spanCtx.TraceID().String())
			}
			if spanCtx.HasSpanID() {
				ctx = logger.ContextWithSpanID(ctx, spanCtx.SpanID().String())
			}

			// 在响应头中返回 traceId，便于客户端关联请求
			if spanCtx.TraceID().IsValid() {
				w.Header().Set(TraceIDHeader, spanCtx.TraceID().String())
			}

			// 使用包装的 ResponseWriter 捕获状态码
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// 执行下一个处理器
			next.ServeHTTP(rw, r.WithContext(ctx))

			// 记录响应状态码
			span.SetAttributes(semconv.HTTPResponseStatusCode(rw.statusCode))

			// 根据状态码设置 span 状态
			if rw.statusCode >= 400 {
				span.SetStatus(codes.Error, http.StatusText(rw.statusCode))
			}
		})
	}
}

// responseWriter 包装 http.ResponseWriter 以捕获状态码和响应大小.
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// SpanFromContext 从 context 获取当前 span.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// StartSpan 在当前 context 中创建新的 span.
// 使用示例:
//
//	ctx, span := tracing.StartSpan(ctx, "my-service", "process-order")
//	defer span.End()
func StartSpan(ctx context.Context, tracerName, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	tracer := otel.Tracer(tracerName)
	return tracer.Start(ctx, spanName, opts...)
}

// AddSpanEvent 向当前 span 添加事件.
func AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetSpanError 设置 span 错误状态.
func SetSpanError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// SetSpanAttributes 设置 span 属性.
func SetSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// InjectHTTPHeaders 将追踪信息注入到 HTTP 请求头.
// 用于向下游服务传播追踪上下文.
// 使用示例:
//
//	req, _ := http.NewRequestWithContext(ctx, "GET", "http://service-b/api", nil)
//	tracing.InjectHTTPHeaders(ctx, req)
//	resp, err := client.Do(req)
func InjectHTTPHeaders(ctx context.Context, req *http.Request) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
}

// TraceID 从 context 获取 trace ID.
func TraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// SpanID 从 context 获取 span ID.
func SpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasSpanID() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// EndpointMiddleware 返回 Endpoint 链路追踪中间件.
// 使用示例:
//
//	endpoint = trace.EndpointMiddleware("my-service", "GetUser")(endpoint)
func EndpointMiddleware(serviceName, operationName string) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (response any, err error) {
			tracer := otel.Tracer(serviceName)
			ctx, span := tracer.Start(ctx, operationName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					attribute.String("service.name", serviceName),
					attribute.String("operation.name", operationName),
				),
			)
			defer span.End()

			response, err = next(ctx, request)

			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}

			return response, err
		}
	}
}

// EndpointTracer 提供可配置的 Endpoint 链路追踪.
type EndpointTracer struct {
	serviceName string
}

// NewEndpointTracer 创建 Endpoint 链路追踪器.
func NewEndpointTracer(serviceName string) *EndpointTracer {
	return &EndpointTracer{
		serviceName: serviceName,
	}
}

// Middleware 返回指定操作的追踪中间件.
func (t *EndpointTracer) Middleware(operationName string) endpoint.Middleware {
	return EndpointMiddleware(t.serviceName, operationName)
}
