// Package grpcx provides gRPC adapters for auth/jwt.
package grpcx

import (
	"context"
	"crypto/subtle"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	servexjwt "github.com/Tsukikage7/servex/v2/auth/jwt"
	"github.com/Tsukikage7/servex/v2/observability/logger"
	transportgrpcx "github.com/Tsukikage7/servex/v2/transport/grpcx"
)

// UnaryServerInterceptor 创建 gRPC 一元服务端拦截器.
func UnaryServerInterceptor(j *servexjwt.JWT) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if IsWhitelisted(ctx, j.Whitelist()) {
			return handler(ctx, req)
		}

		token, err := ExtractToken(ctx)
		if err != nil {
			j.Logger().With(logger.Err(err)).Warn("gRPC 令牌提取失败")
			return nil, status.Error(codes.Unauthenticated, "认证失败")
		}

		claims, err := j.Validate(ctx, token)
		if err != nil {
			j.Logger().With(logger.Err(err)).Warn("gRPC 令牌验证失败")
			return nil, status.Error(codes.Unauthenticated, "认证失败")
		}

		if c, ok := claims.(servexjwt.Claims); ok {
			ctx = servexjwt.ContextWithClaims(ctx, c)
			ctx = servexjwt.ContextWithToken(ctx, token)
		}

		return handler(ctx, req)
	}
}

// UnaryServerInterceptorWithClaims 创建使用自定义 Claims 类型的 gRPC 一元服务端拦截器.
func UnaryServerInterceptorWithClaims(j *servexjwt.JWT, cf servexjwt.ClaimsFactory) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if IsWhitelisted(ctx, j.Whitelist()) {
			return handler(ctx, req)
		}

		token, err := ExtractToken(ctx)
		if err != nil {
			j.Logger().With(logger.Err(err)).Warn("gRPC 令牌提取失败")
			return nil, status.Error(codes.Unauthenticated, "认证失败")
		}

		claims, err := j.ValidateWithClaims(ctx, token, cf())
		if err != nil {
			j.Logger().With(logger.Err(err)).Warn("gRPC 令牌验证失败")
			return nil, status.Error(codes.Unauthenticated, "认证失败")
		}

		if c, ok := claims.(servexjwt.Claims); ok {
			ctx = servexjwt.ContextWithClaims(ctx, c)
			ctx = servexjwt.ContextWithToken(ctx, token)
		}

		return handler(ctx, req)
	}
}

// StreamServerInterceptor 创建 gRPC 流服务端拦截器.
func StreamServerInterceptor(j *servexjwt.JWT) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ss.Context()
		if IsWhitelisted(ctx, j.Whitelist()) {
			return handler(srv, ss)
		}

		token, err := ExtractToken(ctx)
		if err != nil {
			j.Logger().With(logger.Err(err)).Warn("gRPC 流令牌提取失败")
			return status.Error(codes.Unauthenticated, "认证失败")
		}

		claims, err := j.Validate(ctx, token)
		if err != nil {
			j.Logger().With(logger.Err(err)).Warn("gRPC 流令牌验证失败")
			return status.Error(codes.Unauthenticated, "认证失败")
		}

		if c, ok := claims.(servexjwt.Claims); ok {
			ctx = servexjwt.ContextWithClaims(ctx, c)
			ctx = servexjwt.ContextWithToken(ctx, token)
			ss = transportgrpcx.WrapServerStream(ss, ctx)
		}

		return handler(srv, ss)
	}
}

// StreamServerInterceptorWithClaims 创建使用自定义 Claims 类型的 gRPC 流服务端拦截器.
func StreamServerInterceptorWithClaims(j *servexjwt.JWT, cf servexjwt.ClaimsFactory) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ss.Context()
		if IsWhitelisted(ctx, j.Whitelist()) {
			return handler(srv, ss)
		}

		token, err := ExtractToken(ctx)
		if err != nil {
			j.Logger().With(logger.Err(err)).Warn("gRPC 流令牌提取失败")
			return status.Error(codes.Unauthenticated, "认证失败")
		}

		claims, err := j.ValidateWithClaims(ctx, token, cf())
		if err != nil {
			j.Logger().With(logger.Err(err)).Warn("gRPC 流令牌验证失败")
			return status.Error(codes.Unauthenticated, "认证失败")
		}

		if c, ok := claims.(servexjwt.Claims); ok {
			ctx = servexjwt.ContextWithClaims(ctx, c)
			ctx = servexjwt.ContextWithToken(ctx, token)
			ss = transportgrpcx.WrapServerStream(ss, ctx)
		}

		return handler(srv, ss)
	}
}

// ExtractToken 从 gRPC metadata 提取令牌.
func ExtractToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", servexjwt.ErrTokenNotFound
	}
	if authHeaders := md.Get("authorization"); len(authHeaders) > 0 {
		return servexjwt.ExtractTokenFromHeader(authHeaders[0]), nil
	}
	return "", servexjwt.ErrTokenNotFound
}

// IsWhitelisted 检查 gRPC 调用是否命中 JWT 白名单.
func IsWhitelisted(ctx context.Context, whitelist *servexjwt.Whitelist) bool {
	if whitelist == nil {
		return false
	}
	if isInternalService(ctx, whitelist) {
		return true
	}
	if method, ok := grpc.Method(ctx); ok {
		return isGRPCMethodWhitelisted(whitelist, method)
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if paths := md.Get(":path"); len(paths) > 0 {
			return isGRPCMethodWhitelisted(whitelist, paths[0])
		}
	}
	return false
}

func isInternalService(ctx context.Context, whitelist *servexjwt.Whitelist) bool {
	if whitelist.InternalServiceHeader == "" || whitelist.InternalServiceSecret == "" {
		return false
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	services := md.Get(whitelist.InternalServiceHeader)
	if len(services) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(services[0]), []byte(whitelist.InternalServiceSecret)) == 1
}

func isGRPCMethodWhitelisted(whitelist *servexjwt.Whitelist, method string) bool {
	for _, whitelistMethod := range whitelist.GRPCMethods {
		if method == whitelistMethod {
			return true
		}
		if strings.HasPrefix(method, whitelistMethod) &&
			(strings.HasSuffix(whitelistMethod, "/") || len(method) > len(whitelistMethod) && method[len(whitelistMethod)] == '/') {
			return true
		}
	}
	return false
}
