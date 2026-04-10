// Package trace 提供请求链路追踪增强中间件.
// 统一 trace-id 在日志、响应头、下游调用中的传播.
//
// 与 observability/tracing 的关系:
//   - observability/tracing: OTel 全量链路追踪（Span/Export/Jaeger），重依赖
//   - middleware/trace: 轻量级 trace-id 传播 + logger 注入，零外部依赖（仅 UUID）
//
// 两者可共存: 若 OTel span 已存在，本中间件优先使用 OTel 的 traceId/spanId，
// 不再自行生成，避免覆盖.
package trace

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	otelTrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/Tsukikage7/servex/observability/logger"
	"github.com/Tsukikage7/servex/transport/grpcx"
)

// context key 类型，避免与其他包冲突.
type ctxKey int

const (
	traceIDKey ctxKey = iota
)

// Config 链路追踪增强配置.
type Config struct {
	// TraceIDHeader HTTP 响应头中的 trace ID 字段名，默认 "X-Trace-ID"
	TraceIDHeader string
	// PropagateHeaders 需要传播到下游的 header 列表
	PropagateHeaders []string
	// Logger 日志器（自动注入 traceId/spanId 字段）
	Logger logger.Logger
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		TraceIDHeader: "X-Trace-ID",
	}
}

// applyDefaults 将未设置的字段填充为默认值.
func (c *Config) applyDefaults() {
	if c.TraceIDHeader == "" {
		c.TraceIDHeader = "X-Trace-ID"
	}
}

// generateID 生成新的 UUID.
func generateID() string {
	return uuid.New().String()
}

// extractOrGenerateTraceID 从 OTel span 或 header 中提取 traceId，都没有则生成新的.
// 优先级: OTel span > header > 新生成 UUID.
func extractOrGenerateTraceID(ctx context.Context, headerValue string) (traceID, spanID string) {
	// 优先从 OTel span 提取（observability/tracing 中间件已创建）
	span := otelTrace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		traceID = span.SpanContext().TraceID().String()
		if span.SpanContext().HasSpanID() {
			spanID = span.SpanContext().SpanID().String()
		}
		return
	}

	// 其次从 header 提取
	if headerValue != "" {
		traceID = headerValue
		return
	}

	// 最后自行生成
	traceID = generateID()
	return
}

// HTTPMiddleware HTTP 链路追踪中间件.
// 功能：
//  1. 从 OTel span / 请求头提取或生成 trace-id
//  2. 注入到 response header
//  3. 注入到 logger context（后续日志自动带 traceId/spanId）
//  4. 注入到 context 供下游使用
func HTTPMiddleware(cfg *Config) func(http.Handler) http.Handler {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	cfg.applyDefaults()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// 提取或生成 trace-id（优先复用 OTel span）
			traceID, spanID := extractOrGenerateTraceID(ctx, r.Header.Get(cfg.TraceIDHeader))

			// 注入到 context
			ctx = withTraceID(ctx, traceID)
			ctx = logger.ContextWithTraceID(ctx, traceID)
			if spanID != "" {
				ctx = logger.ContextWithSpanID(ctx, spanID)
			}

			// 将带 trace 信息的 logger 存入 context
			if cfg.Logger != nil {
				fields := []logger.Field{
					logger.String("traceId", traceID),
				}
				if spanID != "" {
					fields = append(fields, logger.String("spanId", spanID))
				}
				ctx = logger.NewContext(ctx, cfg.Logger.With(fields...))
			}

			// 设置响应头
			w.Header().Set(cfg.TraceIDHeader, traceID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GRPCUnaryInterceptor gRPC 一元拦截器.
// 从入站 metadata 中提取或生成 trace-id，
// 注入到 context 和响应 metadata.
func GRPCUnaryInterceptor(cfg *Config) grpc.UnaryServerInterceptor {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	cfg.applyDefaults()

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		ctx = injectTraceContext(ctx, cfg)
		return handler(ctx, req)
	}
}

// GRPCStreamInterceptor gRPC 流式拦截器.
// 从入站 metadata 中提取或生成 trace-id，
// 注入到 context 和响应 metadata.
func GRPCStreamInterceptor(cfg *Config) grpc.StreamServerInterceptor {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	cfg.applyDefaults()

	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := injectTraceContext(ss.Context(), cfg)
		wrapped := grpcx.WrapServerStream(ss, ctx)
		return handler(srv, wrapped)
	}
}

// injectTraceContext 从 gRPC metadata 提取或生成 traceId 并注入 context.
func injectTraceContext(ctx context.Context, cfg *Config) context.Context {
	traceKey := grpcMetaKey(cfg.TraceIDHeader)

	// 提取或生成 trace-id（优先复用 OTel span）
	traceID, spanID := extractOrGenerateTraceID(ctx, grpcx.GetMetadataValue(ctx, traceKey))

	ctx = withTraceID(ctx, traceID)
	ctx = logger.ContextWithTraceID(ctx, traceID)
	if spanID != "" {
		ctx = logger.ContextWithSpanID(ctx, spanID)
	}

	// 将带 trace 信息的 logger 存入 context
	if cfg.Logger != nil {
		fields := []logger.Field{
			logger.String("traceId", traceID),
		}
		if spanID != "" {
			fields = append(fields, logger.String("spanId", spanID))
		}
		ctx = logger.NewContext(ctx, cfg.Logger.With(fields...))
	}

	// 设置响应 metadata
	_ = grpc.SetHeader(ctx, metadata.Pairs(traceKey, traceID))

	return ctx
}

// grpcMetaKey 将 HTTP 风格的 header 名转为 gRPC metadata key（小写）.
func grpcMetaKey(header string) string {
	result := make([]byte, len(header))
	for i := range header {
		c := header[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// TraceIDFromContext 从 context 获取 trace ID.
func TraceIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(traceIDKey).(string)
	return id
}

// InjectHTTPHeaders 将 trace context 注入到 HTTP 请求头（用于客户端调用下游）.
func InjectHTTPHeaders(ctx context.Context, req *http.Request) {
	if traceID := TraceIDFromContext(ctx); traceID != "" {
		req.Header.Set("X-Trace-ID", traceID)
	}
}

// InjectGRPCMetadata 将 trace context 注入到 gRPC metadata（用于客户端调用下游）.
func InjectGRPCMetadata(ctx context.Context) context.Context {
	if traceID := TraceIDFromContext(ctx); traceID != "" {
		return grpcx.AppendOutgoingMetadata(ctx, "x-trace-id", traceID)
	}
	return ctx
}

// withTraceID 将 trace ID 存入 context.
func withTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIDKey, id)
}
