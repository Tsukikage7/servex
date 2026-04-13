package useragent

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

// UnaryServerInterceptor 返回一元 gRPC 拦截器，解析 User-Agent 并存入 context.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		ctx = extractAndStore(ctx)
		return handler(ctx, req)
	}
}

// StreamServerInterceptor 返回流 gRPC 拦截器，解析 User-Agent 并存入 context.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := extractAndStore(ss.Context())
		return handler(srv, grpcx.WrapServerStream(ss, ctx))
	}
}

// extractAndStore 从 gRPC metadata 提取 User-Agent 并存入 context.
func extractAndStore(ctx context.Context) context.Context {
	var raw string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(MetadataKeyUserAgent); len(values) > 0 {
			raw = values[0]
		}
	}
	ua := Parse(raw)
	return WithUserAgent(ctx, ua)
}
