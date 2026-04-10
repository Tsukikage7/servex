package jwt

import (
	"context"
	"net/http"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/Tsukikage7/servex/endpoint"
	"github.com/Tsukikage7/servex/observability/logger"
	"github.com/Tsukikage7/servex/transport/grpcx"
)

// ClaimsFactory 是创建 Claims 实例的工厂函数.
type ClaimsFactory func() Claims

// NewSigner 创建签名中间件，用于生成 JWT 令牌并存入上下文.
//
// 此中间件从上下文获取 Claims，签名生成令牌后存入上下文，供后续传输层使用。
// 适用于客户端在发起请求前签名令牌。
//
// 使用示例:
//
//	jwtSrv := jwt.NewJWT(jwt.WithSecretKey("secret"), jwt.WithLogger(log))
//	endpoint = jwt.NewSigner(jwtSrv)(endpoint)
func NewSigner(j *JWT) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (any, error) {
			// 从上下文获取 Claims
			claims, ok := ClaimsFromContext(ctx)
			if !ok {
				// 无 Claims 时直接调用下游
				return next(ctx, request)
			}

			// 生成令牌
			token, err := j.Generate(ctx, claims)
			if err != nil {
				return nil, err
			}

			// 将令牌存入上下文
			ctx = ContextWithToken(ctx, token)

			return next(ctx, request)
		}
	}
}

// NewParser 创建解析中间件，用于验证 JWT 令牌并将 Claims 存入上下文.
//
// 此中间件从上下文或请求中提取令牌，验证后将 Claims 存入上下文。
// 适用于服务端验证传入请求的令牌。
//
// 使用示例:
//
//	jwtSrv := jwt.NewJWT(jwt.WithSecretKey("secret"), jwt.WithLogger(log))
//	endpoint = jwt.NewParser(jwtSrv)(endpoint)
func NewParser(j *JWT) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (any, error) {
			// 检查白名单
			if j.IsWhitelisted(ctx, request) {
				return next(ctx, request)
			}

			// 提取令牌
			token, err := j.ExtractToken(ctx, request)
			if err != nil {
				return nil, err
			}

			// 验证令牌
			claims, err := j.Validate(ctx, token)
			if err != nil {
				return nil, err
			}

			// 将 Claims 存入上下文
			if c, ok := claims.(Claims); ok {
				ctx = ContextWithClaims(ctx, c)
				ctx = ContextWithToken(ctx, token)
			}

			return next(ctx, request)
		}
	}
}

// NewParserWithClaims 创建使用自定义 Claims 类型的解析中间件.
//
// 使用示例:
//
//	jwtSrv := jwt.NewJWT(jwt.WithSecretKey("secret"), jwt.WithLogger(log))
//	endpoint = jwt.NewParserWithClaims(jwtSrv, func() jwt.Claims {
//	    return &CustomClaims{}
//	})(endpoint)
func NewParserWithClaims(j *JWT, cf ClaimsFactory) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (any, error) {
			// 检查白名单
			if j.IsWhitelisted(ctx, request) {
				return next(ctx, request)
			}

			// 提取令牌
			token, err := j.ExtractToken(ctx, request)
			if err != nil {
				return nil, err
			}

			// 验证令牌（使用自定义 Claims 类型）
			claims, err := j.ValidateWithClaims(ctx, token, cf())
			if err != nil {
				return nil, err
			}

			// 将 Claims 存入上下文
			if c, ok := claims.(Claims); ok {
				ctx = ContextWithClaims(ctx, c)
				ctx = ContextWithToken(ctx, token)
			}

			return next(ctx, request)
		}
	}
}

// HTTPMiddleware 创建 HTTP 认证中间件.
//
// 使用示例:
//
//	jwtSrv := jwt.NewJWT(jwt.WithSecretKey("secret"), jwt.WithLogger(log))
//	handler = jwt.HTTPMiddleware(jwtSrv)(handler)
func HTTPMiddleware(j *JWT) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 检查白名单
			if j.IsWhitelisted(r.Context(), r) {
				next.ServeHTTP(w, r)
				return
			}

			// 提取令牌
			token, err := j.ExtractToken(r.Context(), r)
			if err != nil {
				// 不向客户端暴露内部错误细节
				http.Error(w, "认证失败", http.StatusUnauthorized)
				return
			}

			// 验证令牌
			claims, err := j.Validate(r.Context(), token)
			if err != nil {
				// 不向客户端暴露内部错误细节
				http.Error(w, "认证失败", http.StatusUnauthorized)
				return
			}

			// 将 Claims 存入上下文
			if c, ok := claims.(Claims); ok {
				ctx := ContextWithClaims(r.Context(), c)
				ctx = ContextWithToken(ctx, token)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// HTTPMiddlewareWithClaims 创建使用自定义 Claims 类型的 HTTP 认证中间件.
//
// 使用示例:
//
//	jwtSrv := jwt.NewJWT(jwt.WithSecretKey("secret"), jwt.WithLogger(log))
//	handler = jwt.HTTPMiddlewareWithClaims(jwtSrv, func() jwt.Claims {
//	    return &CustomClaims{}
//	})(handler)
func HTTPMiddlewareWithClaims(j *JWT, cf ClaimsFactory) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 检查白名单
			if j.IsWhitelisted(r.Context(), r) {
				next.ServeHTTP(w, r)
				return
			}

			// 提取令牌
			token, err := j.ExtractToken(r.Context(), r)
			if err != nil {
				// 不向客户端暴露内部错误细节
				http.Error(w, "认证失败", http.StatusUnauthorized)
				return
			}

			// 验证令牌（使用自定义 Claims 类型）
			claims, err := j.ValidateWithClaims(r.Context(), token, cf())
			if err != nil {
				// 不向客户端暴露内部错误细节
				http.Error(w, "认证失败", http.StatusUnauthorized)
				return
			}

			// 将 Claims 存入上下文
			if c, ok := claims.(Claims); ok {
				ctx := ContextWithClaims(r.Context(), c)
				ctx = ContextWithToken(ctx, token)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// UnaryServerInterceptor 创建 gRPC 一元服务端拦截器.
//
// 使用示例:
//
//	jwtSrv := jwt.NewJWT(jwt.WithSecretKey("secret"), jwt.WithLogger(log))
//	server := grpc.NewServer(
//	    grpc.UnaryInterceptor(jwt.UnaryServerInterceptor(jwtSrv)),
//	)
func UnaryServerInterceptor(j *JWT) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// 检查白名单
		if j.IsWhitelisted(ctx, req) {
			return handler(ctx, req)
		}

		// 提取令牌
		token, err := j.ExtractToken(ctx, req)
		if err != nil {
			// 不向客户端暴露内部错误细节，仅记录日志
			j.opts.logger.With(logger.Err(err)).Warn("[JWT] gRPC 令牌提取失败")
			return nil, status.Error(codes.Unauthenticated, "认证失败")
		}

		// 验证令牌
		claims, err := j.Validate(ctx, token)
		if err != nil {
			// 不向客户端暴露内部错误细节，仅记录日志
			j.opts.logger.With(logger.Err(err)).Warn("[JWT] gRPC 令牌验证失败")
			return nil, status.Error(codes.Unauthenticated, "认证失败")
		}

		// 将 Claims 存入上下文
		if c, ok := claims.(Claims); ok {
			ctx = ContextWithClaims(ctx, c)
			ctx = ContextWithToken(ctx, token)
		}

		return handler(ctx, req)
	}
}

// UnaryServerInterceptorWithClaims 创建使用自定义 Claims 类型的 gRPC 一元服务端拦截器.
func UnaryServerInterceptorWithClaims(j *JWT, cf ClaimsFactory) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// 检查白名单
		if j.IsWhitelisted(ctx, req) {
			return handler(ctx, req)
		}

		// 提取令牌
		token, err := j.ExtractToken(ctx, req)
		if err != nil {
			// 不向客户端暴露内部错误细节，仅记录日志
			j.opts.logger.With(logger.Err(err)).Warn("[JWT] gRPC 令牌提取失败")
			return nil, status.Error(codes.Unauthenticated, "认证失败")
		}

		// ��证令牌（使用自定义 Claims 类型）
		claims, err := j.ValidateWithClaims(ctx, token, cf())
		if err != nil {
			// 不向客户端暴露内部错误细节，仅记录日志
			j.opts.logger.With(logger.Err(err)).Warn("[JWT] gRPC 令牌验证失败")
			return nil, status.Error(codes.Unauthenticated, "认证失败")
		}

		// 将 Claims 存入上下文
		if c, ok := claims.(Claims); ok {
			ctx = ContextWithClaims(ctx, c)
			ctx = ContextWithToken(ctx, token)
		}

		return handler(ctx, req)
	}
}

// StreamServerInterceptor 创建 gRPC 流服务端拦截器.
//
// 使用示例:
//
//	jwtSrv := jwt.NewJWT(jwt.WithSecretKey("secret"), jwt.WithLogger(log))
//	server := grpc.NewServer(
//	    grpc.StreamInterceptor(jwt.StreamServerInterceptor(jwtSrv)),
//	)
func StreamServerInterceptor(j *JWT) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ss.Context()

		// 检查白名单
		if j.IsWhitelisted(ctx, nil) {
			return handler(srv, ss)
		}

		// 提取令牌
		token, err := j.ExtractToken(ctx, nil)
		if err != nil {
			// 不向客户端暴露内部错误细节，仅记录日志
			j.opts.logger.With(logger.Err(err)).Warn("[JWT] gRPC 流令牌提取失败")
			return status.Error(codes.Unauthenticated, "认证失败")
		}

		// 验证令牌
		claims, err := j.Validate(ctx, token)
		if err != nil {
			// 不向客户端暴露内部错误细节，仅记录日志
			j.opts.logger.With(logger.Err(err)).Warn("[JWT] gRPC 流令牌验证失败")
			return status.Error(codes.Unauthenticated, "认证失败")
		}

		// 创建带有 Claims 的包装流
		if c, ok := claims.(Claims); ok {
			ctx = ContextWithClaims(ctx, c)
			ctx = ContextWithToken(ctx, token)
			ss = grpcx.WrapServerStream(ss, ctx)
		}

		return handler(srv, ss)
	}
}

// StreamServerInterceptorWithClaims 创建使用自定义 Claims 类型的 gRPC 流服务端拦截器.
func StreamServerInterceptorWithClaims(j *JWT, cf ClaimsFactory) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ss.Context()

		// 检查白名单
		if j.IsWhitelisted(ctx, nil) {
			return handler(srv, ss)
		}

		// 提取令牌
		token, err := j.ExtractToken(ctx, nil)
		if err != nil {
			// 不向客户端暴露内部错误细节，仅记录日志
			j.opts.logger.With(logger.Err(err)).Warn("[JWT] gRPC 流令牌提取失败")
			return status.Error(codes.Unauthenticated, "认证失败")
		}

		// 验证令牌（使用自定义 Claims 类型）
		claims, err := j.ValidateWithClaims(ctx, token, cf())
		if err != nil {
			// 不向客户端暴露内部错误细节，仅记录日志
			j.opts.logger.With(logger.Err(err)).Warn("[JWT] gRPC 流令牌验证失败")
			return status.Error(codes.Unauthenticated, "认证失败")
		}

		// 创建带有 Claims 的包装流
		if c, ok := claims.(Claims); ok {
			ctx = ContextWithClaims(ctx, c)
			ctx = ContextWithToken(ctx, token)
			ss = grpcx.WrapServerStream(ss, ctx)
		}

		return handler(srv, ss)
	}
}

// ExtractToken 从请求中提取令牌（独立函数）.
func ExtractToken(ctx context.Context, req any) (string, error) {
	// 从 gRPC metadata 提取
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if authHeaders := md.Get("authorization"); len(authHeaders) > 0 {
			return extractTokenFromHeader(authHeaders[0]), nil
		}
	}

	// 从 HTTP 请求提取
	if httpReq, ok := req.(*http.Request); ok {
		if auth := httpReq.Header.Get("Authorization"); auth != "" {
			return extractTokenFromHeader(auth), nil
		}
	}

	// 从上下文提取
	if token, ok := TokenFromContext(ctx); ok {
		return token, nil
	}

	return "", ErrTokenNotFound
}

// extractTokenFromHeader 从 Authorization Header 提取令牌.
func extractTokenFromHeader(header string) string {
	// 移除 Bearer 前缀
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return strings.TrimSpace(header)
}
