package deviceinfo

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/Tsukikage7/servex/v2/transport/grpcx"
)

const (
	// MetadataKeyUserAgent User-Agent 的 gRPC metadata 键名.
	MetadataKeyUserAgent = "user-agent"
	// MetadataKeySecCHUA Sec-CH-UA 的 gRPC metadata 键名.
	MetadataKeySecCHUA = "sec-ch-ua"
	// MetadataKeySecCHUAMobile Sec-CH-UA-Mobile 的 gRPC metadata 键名.
	MetadataKeySecCHUAMobile = "sec-ch-ua-mobile"
	// MetadataKeySecCHUAPlatform Sec-CH-UA-Platform 的 gRPC metadata 键名.
	MetadataKeySecCHUAPlatform = "sec-ch-ua-platform"
	// MetadataKeySecCHUAPlatformVersion Sec-CH-UA-Platform-Version 的 gRPC metadata 键名.
	MetadataKeySecCHUAPlatformVersion = "sec-ch-ua-platform-version"
	// MetadataKeySecCHUAArch Sec-CH-UA-Arch 的 gRPC metadata 键名.
	MetadataKeySecCHUAArch = "sec-ch-ua-arch"
	// MetadataKeySecCHUAModel Sec-CH-UA-Model 的 gRPC metadata 键名.
	MetadataKeySecCHUAModel = "sec-ch-ua-model"
	// MetadataKeyDeviceMemory Device-Memory 的 gRPC metadata 键名.
	MetadataKeyDeviceMemory = "device-memory"
	// MetadataKeyDPR DPR 的 gRPC metadata 键名.
	MetadataKeyDPR = "dpr"
)

// UnaryServerInterceptor 返回一元 gRPC 拦截器，解析设备信息并存入 context.
func UnaryServerInterceptor(opts ...Option) grpc.UnaryServerInterceptor {
	parser := New(opts...)

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		ctx = parseAndStore(ctx, parser)
		return handler(ctx, req)
	}
}

// StreamServerInterceptor 返回流 gRPC 拦截器，解析设备信息并存入 context.
func StreamServerInterceptor(opts ...Option) grpc.StreamServerInterceptor {
	parser := New(opts...)

	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := parseAndStore(ss.Context(), parser)
		return handler(srv, grpcx.WrapServerStream(ss, ctx))
	}
}

// parseAndStore 从 gRPC metadata 解析设备信息并存入 context.
func parseAndStore(ctx context.Context, parser *Parser) context.Context {
	headers := Headers{}

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		headers.UserAgent = getMetadataValue(md, MetadataKeyUserAgent)
		headers.SecCHUA = getMetadataValue(md, MetadataKeySecCHUA)
		headers.SecCHUAMobile = getMetadataValue(md, MetadataKeySecCHUAMobile)
		headers.SecCHUAPlatform = getMetadataValue(md, MetadataKeySecCHUAPlatform)
		headers.SecCHUAPlatformVersion = getMetadataValue(md, MetadataKeySecCHUAPlatformVersion)
		headers.SecCHUAArch = getMetadataValue(md, MetadataKeySecCHUAArch)
		headers.SecCHUAModel = getMetadataValue(md, MetadataKeySecCHUAModel)
		headers.DeviceMemory = getMetadataValue(md, MetadataKeyDeviceMemory)
		headers.DPR = getMetadataValue(md, MetadataKeyDPR)
	}

	deviceInfo := parser.Parse(headers)
	return WithInfo(ctx, deviceInfo)
}

// getMetadataValue 从 metadata 获取单个值.
func getMetadataValue(md metadata.MD, key string) string {
	if values := md.Get(key); len(values) > 0 {
		return values[0]
	}
	return ""
}
