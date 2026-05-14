package grpcx

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/Tsukikage7/servex/v2/auth"
	transportgrpcx "github.com/Tsukikage7/servex/v2/transport/grpcx"
)

// UnaryServerInterceptor 返回 gRPC 一元服务器认证拦截器.
//
// 示例:
//
//	authenticator := jwt.NewAuthenticator(jwtSrv)
//	srv := grpc.NewServer(
//	    grpc.UnaryInterceptor(grpcx.UnaryServerInterceptor(authenticator)),
//	)
func UnaryServerInterceptor(authenticator auth.Authenticator, opts ...auth.Option) grpc.UnaryServerInterceptor {
	if authenticator == nil {
		panic("auth: 认证器不能为空")
	}

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		authCtx, skipped, err := auth.AuthenticateRequest(
			ctx,
			req,
			targetForMethod(info.FullMethod),
			authenticator,
			DefaultCredentialsExtractor,
			opts...,
		)
		if skipped {
			return handler(ctx, req)
		}
		if err != nil {
			return nil, grpcErrorForAuthError(err)
		}

		return handler(authCtx, req)
	}
}

// StreamServerInterceptor 返回 gRPC 流服务器认证拦截器.
func StreamServerInterceptor(authenticator auth.Authenticator, opts ...auth.Option) grpc.StreamServerInterceptor {
	if authenticator == nil {
		panic("auth: 认证器不能为空")
	}

	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ss.Context()
		authCtx, skipped, err := auth.AuthenticateRequest(
			ctx,
			nil,
			targetForMethod(info.FullMethod),
			authenticator,
			DefaultCredentialsExtractor,
			opts...,
		)
		if skipped {
			return handler(srv, ss)
		}
		if err != nil {
			return grpcErrorForAuthError(err)
		}

		wrapped := transportgrpcx.WrapServerStream(ss, authCtx)
		return handler(srv, wrapped)
	}
}

// DefaultCredentialsExtractor 默认的 gRPC 凭据提取器.
func DefaultCredentialsExtractor(ctx context.Context, _ any) (*auth.Credentials, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, auth.ErrCredentialsNotFound
	}

	// 1. authorization (Bearer)
	if vals := md.Get(auth.GRPCAuthorizationMetadata); len(vals) > 0 {
		header := vals[0]
		if strings.HasPrefix(header, auth.BearerPrefix) {
			return &auth.Credentials{
				Type:  auth.CredentialTypeBearer,
				Token: strings.TrimPrefix(header, auth.BearerPrefix),
			}, nil
		}
		if strings.HasPrefix(strings.ToLower(header), "bearer ") {
			return &auth.Credentials{
				Type:  auth.CredentialTypeBearer,
				Token: header[7:],
			}, nil
		}
	}

	// 2. x-api-key
	if vals := md.Get(auth.GRPCAPIKeyMetadata); len(vals) > 0 {
		return &auth.Credentials{
			Type:  auth.CredentialTypeAPIKey,
			Token: vals[0],
		}, nil
	}

	return nil, auth.ErrCredentialsNotFound
}

func targetForMethod(fullMethod string) auth.Target {
	return auth.Target{
		Resource: fullMethod,
		Method:   fullMethod,
	}
}

func grpcErrorForAuthError(err error) error {
	if auth.IsForbidden(err) {
		return status.Error(codes.PermissionDenied, "权限被拒绝")
	}
	return status.Error(codes.Unauthenticated, "认证失败")
}

// BearerExtractor 仅提取 Bearer Token.
func BearerExtractor(ctx context.Context, _ any) (*auth.Credentials, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, auth.ErrCredentialsNotFound
	}

	vals := md.Get(auth.GRPCAuthorizationMetadata)
	if len(vals) == 0 {
		return nil, auth.ErrCredentialsNotFound
	}

	header := vals[0]
	// 统一使用不区分大小写的前缀匹配
	lower := strings.ToLower(header)
	if !strings.HasPrefix(lower, "bearer ") {
		return nil, auth.ErrCredentialsNotFound
	}

	token := strings.TrimSpace(header[7:])
	return &auth.Credentials{
		Type:  auth.CredentialTypeBearer,
		Token: token,
	}, nil
}

// SkipMethods 返回跳过指定 gRPC 方法的 Skipper.
func SkipMethods(methods ...string) auth.Skipper {
	methodSet := make(map[string]bool)
	for _, m := range methods {
		methodSet[m] = true
	}
	return func(ctx context.Context, _ any) bool {
		method, ok := grpc.Method(ctx)
		if !ok {
			return false
		}
		return methodSet[method]
	}
}
