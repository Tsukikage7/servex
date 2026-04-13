package referer

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/Tsukikage7/servex/v2/transport/grpcx"
)

const (
	// MetadataKeyReferer gRPC metadata 中的 Referer 键名.
	MetadataKeyReferer = "referer"
)

// GRPCOption gRPC 配置选项.
type GRPCOption func(*grpcOptions)

type grpcOptions struct {
	currentHost string
}

// WithGRPCCurrentHost 设置当前站点域名.
func WithGRPCCurrentHost(host string) GRPCOption {
	return func(o *grpcOptions) {
		o.currentHost = host
	}
}

// UnaryServerInterceptor 返回一元 gRPC 拦截器，解析 Referer 并存入 context.
func UnaryServerInterceptor(opts ...GRPCOption) grpc.UnaryServerInterceptor {
	o := &grpcOptions{}
	for _, opt := range opts {
		opt(o)
	}

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		ctx = extractAndStore(ctx, o.currentHost)
		return handler(ctx, req)
	}
}

// StreamServerInterceptor 返回流 gRPC 拦截器，解析 Referer 并存入 context.
func StreamServerInterceptor(opts ...GRPCOption) grpc.StreamServerInterceptor {
	o := &grpcOptions{}
	for _, opt := range opts {
		opt(o)
	}

	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := extractAndStore(ss.Context(), o.currentHost)
		return handler(srv, grpcx.WrapServerStream(ss, ctx))
	}
}

// extractAndStore 从 gRPC metadata 提取 Referer 并存入 context.
func extractAndStore(ctx context.Context, currentHost string) context.Context {
	var raw string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(MetadataKeyReferer); len(values) > 0 {
			raw = values[0]
		}
	}

	var ref *Referer
	if currentHost != "" {
		ref = ParseWithHost(raw, currentHost)
	} else {
		ref = Parse(raw)
	}
	return WithReferer(ctx, ref)
}
