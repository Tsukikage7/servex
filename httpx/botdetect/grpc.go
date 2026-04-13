package botdetect

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/Tsukikage7/servex/v2/transport/grpcx"
)

const (
	// MetadataKeyUserAgent gRPC metadata 中的 User-Agent 键名.
	MetadataKeyUserAgent = "user-agent"
)

// UnaryServerInterceptor 返回一元 gRPC 拦截器，检测机器人并存入 context.
func UnaryServerInterceptor(opts ...Option) grpc.UnaryServerInterceptor {
	detector := New(opts...)

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		ctx = detectAndStore(ctx, detector)
		return handler(ctx, req)
	}
}

// StreamServerInterceptor 返回流 gRPC 拦截器，检测机器人并存入 context.
func StreamServerInterceptor(opts ...Option) grpc.StreamServerInterceptor {
	detector := New(opts...)

	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := detectAndStore(ss.Context(), detector)
		return handler(srv, grpcx.WrapServerStream(ss, ctx))
	}
}

// detectAndStore 从 gRPC metadata 提取 User-Agent 并检测机器人.
func detectAndStore(ctx context.Context, detector *Detector) context.Context {
	var userAgent string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(MetadataKeyUserAgent); len(values) > 0 {
			userAgent = values[0]
		}
	}
	result := detector.Detect(userAgent)
	return WithResult(ctx, result)
}
