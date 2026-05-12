package auth

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/Tsukikage7/servex/v2/transport/grpcx"
)

// UnaryServerInterceptor 返回 gRPC 一元服务器认证拦截器.
//
// 示例:
//
//	authenticator := jwt.NewAuthenticator(jwtSrv)
//	srv := grpc.NewServer(
//	    grpc.UnaryInterceptor(auth.UnaryServerInterceptor(authenticator)),
//	)
func UnaryServerInterceptor(authenticator Authenticator, opts ...Option) grpc.UnaryServerInterceptor {
	if authenticator == nil {
		panic("auth: 认证器不能为空")
	}

	o := defaultOptions(authenticator)
	for _, opt := range opts {
		opt(o)
	}

	if o.credentialsExtractor == nil {
		o.credentialsExtractor = DefaultGRPCCredentialsExtractor
	}

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// 检查是否跳过
		if o.skipper != nil && o.skipper(ctx, req) {
			return handler(ctx, req)
		}

		authCtx, _, err := authenticateAndAuthorize(ctx, req, o, grpcTarget(info.FullMethod))
		if err != nil {
			return nil, grpcErrorForAuthError(handleError(ctx, err, o))
		}

		return handler(authCtx, req)
	}
}

// StreamServerInterceptor 返回 gRPC 流服务器认证拦截器.
func StreamServerInterceptor(authenticator Authenticator, opts ...Option) grpc.StreamServerInterceptor {
	if authenticator == nil {
		panic("auth: 认证器不能为空")
	}

	o := defaultOptions(authenticator)
	for _, opt := range opts {
		opt(o)
	}

	if o.credentialsExtractor == nil {
		o.credentialsExtractor = DefaultGRPCCredentialsExtractor
	}

	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ss.Context()

		// 检查是否跳过
		if o.skipper != nil && o.skipper(ctx, nil) {
			return handler(srv, ss)
		}

		authCtx, _, err := authenticateAndAuthorize(ctx, nil, o, grpcTarget(info.FullMethod))
		if err != nil {
			return grpcErrorForAuthError(handleError(ctx, err, o))
		}

		wrapped := grpcx.WrapServerStream(ss, authCtx)
		return handler(srv, wrapped)
	}
}

// DefaultGRPCCredentialsExtractor 默认的 gRPC 凭据提取器.
func DefaultGRPCCredentialsExtractor(ctx context.Context, _ any) (*Credentials, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, ErrCredentialsNotFound
	}

	// 1. authorization (Bearer)
	if vals := md.Get(GRPCAuthorizationMetadata); len(vals) > 0 {
		auth := vals[0]
		if strings.HasPrefix(auth, BearerPrefix) {
			return &Credentials{
				Type:  CredentialTypeBearer,
				Token: strings.TrimPrefix(auth, BearerPrefix),
			}, nil
		}
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			return &Credentials{
				Type:  CredentialTypeBearer,
				Token: auth[7:],
			}, nil
		}
	}

	// 2. x-api-key
	if vals := md.Get(GRPCAPIKeyMetadata); len(vals) > 0 {
		return &Credentials{
			Type:  CredentialTypeAPIKey,
			Token: vals[0],
		}, nil
	}

	return nil, ErrCredentialsNotFound
}

func grpcTarget(fullMethod string) Target {
	return Target{
		Resource: fullMethod,
		Method:   fullMethod,
	}
}

func grpcErrorForAuthError(err error) error {
	if IsForbidden(err) {
		return status.Error(codes.PermissionDenied, "权限被拒绝")
	}
	return status.Error(codes.Unauthenticated, "认证失败")
}

// GRPCBearerExtractor 仅提取 Bearer Token.
func GRPCBearerExtractor(ctx context.Context, _ any) (*Credentials, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, ErrCredentialsNotFound
	}

	vals := md.Get(GRPCAuthorizationMetadata)
	if len(vals) == 0 {
		return nil, ErrCredentialsNotFound
	}

	auth := vals[0]
	// 统一使用不区分大小写的前缀匹配
	lower := strings.ToLower(auth)
	if !strings.HasPrefix(lower, "bearer ") {
		return nil, ErrCredentialsNotFound
	}

	token := strings.TrimSpace(auth[7:])
	return &Credentials{
		Type:  CredentialTypeBearer,
		Token: token,
	}, nil
}

// GRPCSkipMethods 返回跳过指定 gRPC 方法的 Skipper.
func GRPCSkipMethods(methods ...string) Skipper {
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
