package clientip

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"

	"github.com/Tsukikage7/servex/v2/transport/grpcx"
)

const (
	// ForwardedForMetadata X-Forwarded-For 对应的 gRPC metadata 键名.
	ForwardedForMetadata = "x-forwarded-for"
	// RealIPMetadata X-Real-IP 对应的 gRPC metadata 键名.
	RealIPMetadata = "x-real-ip"
)

// UnaryServerInterceptor 创建一元 gRPC 客户端 IP 提取拦截器.
//
// 提取优先级:
//  1. x-forwarded-for metadata
//  2. x-real-ip metadata
//  3. peer.FromContext()
//
// 示例:
//
//	grpc.NewServer(
//	    grpc.ChainUnaryInterceptor(
//	        clientip.UnaryServerInterceptor(),
//	    ),
//	)
func UnaryServerInterceptor(opts ...Option) grpc.UnaryServerInterceptor {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ip := extractFromGRPC(ctx, o)
		ctx = WithIP(ctx, ip)

		// 如果配置了地理位置解析器
		if o.geoResolver != nil && ip.Address != "" {
			if geo, err := o.geoResolver.Lookup(ip.Address); err == nil && geo != nil {
				ctx = WithGeoInfo(ctx, geo)
			}
		}

		return handler(ctx, req)
	}
}

// StreamServerInterceptor 创建流式 gRPC 客户端 IP 提取拦截器.
func StreamServerInterceptor(opts ...Option) grpc.StreamServerInterceptor {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		ip := extractFromGRPC(ctx, o)
		ctx = WithIP(ctx, ip)

		// 如果配置了地理位置解析器
		if o.geoResolver != nil && ip.Address != "" {
			if geo, err := o.geoResolver.Lookup(ip.Address); err == nil && geo != nil {
				ctx = WithGeoInfo(ctx, geo)
			}
		}

		return handler(srv, grpcx.WrapServerStream(ss, ctx))
	}
}

// extractFromGRPC 从 gRPC context 中提取客户端 IP.
func extractFromGRPC(ctx context.Context, o *options) *IP {
	// 获取 peer 地址
	var peerIP *IP
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		peerIP = ParseIP(p.Addr.String())
	}

	// 如果不信任所有代理，检查 peer 是否是可信代理
	if peerIP != nil && !o.trustAllProxies && !o.isTrustedProxy(peerIP.Address) {
		return peerIP
	}

	// 获取 metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		if peerIP != nil {
			return peerIP
		}
		return &IP{}
	}

	// 尝试从 x-forwarded-for 提取
	forwardedKey := strings.ToLower(o.forwardedHeader)
	if values := md.Get(forwardedKey); len(values) > 0 {
		xff := values[0]
		var ipStr string
		if o.trustAllProxies {
			ipStr = ParseXForwardedFor(xff)
		} else {
			ipStr = ParseXForwardedForWithTrust(xff, o.isTrustedProxy)
		}
		if ipStr != "" && IsValidIP(ipStr) {
			return &IP{Address: ipStr, Raw: xff}
		}
	}

	// 尝试从 x-real-ip 提取
	realIPKey := strings.ToLower(o.realIPHeader)
	if values := md.Get(realIPKey); len(values) > 0 {
		ipStr := ParseIP(values[0]).Address
		if ipStr != "" && IsValidIP(ipStr) {
			return &IP{Address: ipStr, Raw: values[0]}
		}
	}

	// 使用 peer 地址
	if peerIP != nil {
		return peerIP
	}

	return &IP{}
}

// GRPCKeyFunc 返回用于限流等场景的 gRPC 键提取函数.
//
// 从 context 中获取已解析的客户端 IP.
// 需要配合 UnaryServerInterceptor 使用.
func GRPCKeyFunc() func(ctx context.Context, info *grpc.UnaryServerInfo) string {
	return func(ctx context.Context, info *grpc.UnaryServerInfo) string {
		return GetIP(ctx)
	}
}
