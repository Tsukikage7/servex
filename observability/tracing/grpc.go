package tracing

import (
	"context"
	"errors"
	"io"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// metadataCarrier 实现 propagation.TextMapCarrier 接口.
type metadataCarrier metadata.MD

func (mc metadataCarrier) Get(key string) string {
	vals := metadata.MD(mc).Get(key)
	if len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func (mc metadataCarrier) Set(key, value string) {
	metadata.MD(mc).Set(key, value)
}

func (mc metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(mc))
	for k := range mc {
		keys = append(keys, k)
	}
	return keys
}

// TraceIDMetadataKey gRPC metadata 中 traceId 的键名.
const TraceIDMetadataKey = "x-trace-id"

// UnaryServerInterceptor 返回 gRPC 一元服务端拦截器.
// 拦截器会自动从请求 metadata 提取或生成 traceId，并通过响应 header 返回.
// traceId 同时作为请求的唯一标识（requestId），可通过 TraceID(ctx) 获取.
// 使用示例:
//
//	server := grpc.NewServer(
//	    grpc.UnaryInterceptor(trace.UnaryServerInterceptor("my-service")),
//	)
//	// 在服务实现中获取 traceId
//	func (s *Server) GetUser(ctx context.Context, req *pb.Request) (*pb.Response, error) {
//	    traceId := trace.TraceID(ctx)
//	    // ...
//	}
func UnaryServerInterceptor(serviceName string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// 从 metadata 提取上下文
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.MD{}
		}
		ctx = otel.GetTextMapPropagator().Extract(ctx, metadataCarrier(md))

		tracer := otel.Tracer(serviceName)
		ctx, span := tracer.Start(ctx, info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.RPCSystemGRPC,
				semconv.RPCService(serviceName),
				semconv.RPCMethod(info.FullMethod),
			),
		)
		defer span.End()

		// 注入 trace 信息到 context（供 middleware/trace 读取）
		spanCtx := span.SpanContext()
		if spanCtx.HasTraceID() {
			ctx = logger.ContextWithTraceID(ctx, spanCtx.TraceID().String())
		}
		if spanCtx.HasSpanID() {
			ctx = logger.ContextWithSpanID(ctx, spanCtx.SpanID().String())
		}

		// 在响应 header 中返回 traceId，便于客户端关联请求
		if spanCtx.TraceID().IsValid() {
			header := metadata.Pairs(TraceIDMetadataKey, spanCtx.TraceID().String())
			_ = grpc.SetHeader(ctx, header)
		}

		// 执行处理器
		resp, err := handler(ctx, req)

		// 记录状态
		if err != nil {
			s, _ := status.FromError(err)
			span.SetAttributes(attribute.String("rpc.grpc.status_code", s.Code().String()))
			span.SetStatus(codes.Error, s.Message())
			span.RecordError(err)
		} else {
			span.SetAttributes(attribute.String("rpc.grpc.status_code", "OK"))
		}

		return resp, err
	}
}

// StreamServerInterceptor 返回 gRPC 流式服务端拦截器.
// 拦截器会自动从请求 metadata 提取或生成 traceId，并通过响应 header 返回.
// traceId 同时作为请求的唯一标识（requestId），可通过 TraceID(ctx) 获取.
// 使用示例:
//
//	server := grpc.NewServer(
//	    grpc.StreamInterceptor(trace.StreamServerInterceptor("my-service")),
//	)
func StreamServerInterceptor(serviceName string) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ss.Context()

		// 从 metadata 提取上下文
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.MD{}
		}
		ctx = otel.GetTextMapPropagator().Extract(ctx, metadataCarrier(md))

		tracer := otel.Tracer(serviceName)
		ctx, span := tracer.Start(ctx, info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.RPCSystemGRPC,
				semconv.RPCService(serviceName),
				semconv.RPCMethod(info.FullMethod),
				attribute.Bool("rpc.grpc.is_client_stream", info.IsClientStream),
				attribute.Bool("rpc.grpc.is_server_stream", info.IsServerStream),
			),
		)
		defer span.End()

		// 注入 trace 信息到 context（供 middleware/trace 读取）
		spanCtx := span.SpanContext()
		if spanCtx.HasTraceID() {
			ctx = logger.ContextWithTraceID(ctx, spanCtx.TraceID().String())
		}
		if spanCtx.HasSpanID() {
			ctx = logger.ContextWithSpanID(ctx, spanCtx.SpanID().String())
		}

		// 在响应 header 中返回 traceId，便于客户端关联请求
		if spanCtx.TraceID().IsValid() {
			header := metadata.Pairs(TraceIDMetadataKey, spanCtx.TraceID().String())
			if err := ss.SendHeader(header); err != nil {
				// SendHeader 可能因流已关闭等原因失败，仅记录但不中断处理
				span.RecordError(err)
			}
		}

		// 包装 ServerStream 以传递新的 context
		wrappedStream := &serverStreamWrapper{
			ServerStream: ss,
			ctx:          ctx,
		}

		// 执行处理器
		err := handler(srv, wrappedStream)

		// 记录状态
		if err != nil {
			s, _ := status.FromError(err)
			span.SetAttributes(attribute.String("rpc.grpc.status_code", s.Code().String()))
			span.SetStatus(codes.Error, s.Message())
			span.RecordError(err)
		} else {
			span.SetAttributes(attribute.String("rpc.grpc.status_code", "OK"))
		}

		return err
	}
}

// serverStreamWrapper 包装 grpc.ServerStream 以传递追踪上下文.
type serverStreamWrapper struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *serverStreamWrapper) Context() context.Context {
	return w.ctx
}

// UnaryClientInterceptor 返回 gRPC 一元客户端拦截器.
// 使用示例:
//
//	conn, err := grpc.Dial(address,
//	    grpc.WithUnaryInterceptor(tracing.UnaryClientInterceptor("my-service")),
//	)
func UnaryClientInterceptor(serviceName string) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		tracer := otel.Tracer(serviceName)
		ctx, span := tracer.Start(ctx, method,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				semconv.RPCSystemGRPC,
				semconv.RPCService(serviceName),
				semconv.RPCMethod(method),
			),
		)
		defer span.End()

		// 注入追踪信息到 metadata
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.MD{}
		} else {
			md = md.Copy()
		}
		otel.GetTextMapPropagator().Inject(ctx, metadataCarrier(md))
		ctx = metadata.NewOutgoingContext(ctx, md)

		// 执行调用
		err := invoker(ctx, method, req, reply, cc, opts...)

		// 记录状态
		if err != nil {
			s, _ := status.FromError(err)
			span.SetAttributes(attribute.String("rpc.grpc.status_code", s.Code().String()))
			span.SetStatus(codes.Error, s.Message())
			span.RecordError(err)
		} else {
			span.SetAttributes(attribute.String("rpc.grpc.status_code", "OK"))
		}

		return err
	}
}

// StreamClientInterceptor 返回 gRPC 流式客户端拦截器.
// 使用示例:
//
//	conn, err := grpc.Dial(address,
//	    grpc.WithStreamInterceptor(tracing.StreamClientInterceptor("my-service")),
//	)
func StreamClientInterceptor(serviceName string) grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		tracer := otel.Tracer(serviceName)
		ctx, span := tracer.Start(ctx, method,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				semconv.RPCSystemGRPC,
				semconv.RPCService(serviceName),
				semconv.RPCMethod(method),
				attribute.Bool("rpc.grpc.is_client_stream", desc.ClientStreams),
				attribute.Bool("rpc.grpc.is_server_stream", desc.ServerStreams),
			),
		)

		// 注入追踪信息到 metadata
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.MD{}
		} else {
			md = md.Copy()
		}
		otel.GetTextMapPropagator().Inject(ctx, metadataCarrier(md))
		ctx = metadata.NewOutgoingContext(ctx, md)

		// 执行调用
		clientStream, err := streamer(ctx, desc, cc, method, opts...)

		// 记录错误（如果有）
		if err != nil {
			s, _ := status.FromError(err)
			span.SetAttributes(attribute.String("rpc.grpc.status_code", s.Code().String()))
			span.SetStatus(codes.Error, s.Message())
			span.RecordError(err)
			span.End()
			return nil, err
		}

		// 包装 ClientStream 以在流结束时关闭 span
		return &clientStreamWrapper{
			ClientStream: clientStream,
			span:         span,
		}, nil
	}
}

// clientStreamWrapper 包装 grpc.ClientStream 以在流结束时关闭 span.
type clientStreamWrapper struct {
	grpc.ClientStream
	span trace.Span
}

func (w *clientStreamWrapper) RecvMsg(m any) error {
	err := w.ClientStream.RecvMsg(m)
	if err != nil {
		// 流结束或错误时关闭 span
		if errors.Is(err, io.EOF) {
			w.span.SetAttributes(attribute.String("rpc.grpc.status_code", "OK"))
		} else {
			s, _ := status.FromError(err)
			w.span.SetAttributes(attribute.String("rpc.grpc.status_code", s.Code().String()))
			w.span.SetStatus(codes.Error, s.Message())
			w.span.RecordError(err)
		}
		w.span.End()
	}
	return err
}

// InjectGRPCMetadata 将追踪信息注入到 gRPC metadata.
// 用于手动传播追踪上下文.
// 使用示例:
//
//	ctx = tracing.InjectGRPCMetadata(ctx)
//	client.SomeMethod(ctx, req)
func InjectGRPCMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.MD{}
	} else {
		md = md.Copy()
	}
	otel.GetTextMapPropagator().Inject(ctx, metadataCarrier(md))
	return metadata.NewOutgoingContext(ctx, md)
}

// ExtractGRPCMetadata 从 gRPC metadata 提取追踪信息.
// 用于手动提取追踪上下文.
// 使用示例:
//
//	ctx = tracing.ExtractGRPCMetadata(ctx)
//	span := tracing.SpanFromContext(ctx)
func ExtractGRPCMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, metadataCarrier(md))
}
