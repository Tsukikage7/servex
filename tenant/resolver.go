package tenant

import (
	"context"
	"net/http"
	"strings"

	"google.golang.org/grpc/metadata"

	"github.com/Tsukikage7/servex/v2/auth"
)

// Resolver 从令牌解析租户.
type Resolver interface {
	Resolve(ctx context.Context, token string) (Tenant, error)
}

// TokenExtractor 从请求中提取租户令牌.
type TokenExtractor func(ctx context.Context, request any) (string, error)

// BearerTokenExtractor 从 Authorization: Bearer <token> 提取令牌.
func BearerTokenExtractor() TokenExtractor {
	return func(_ context.Context, request any) (string, error) {
		r, ok := request.(*http.Request)
		if !ok {
			return "", ErrMissingToken
		}
		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			return "", ErrMissingToken
		}
		return strings.TrimPrefix(auth, "Bearer "), nil
	}
}

// HeaderTokenExtractor 从指定 HTTP 请求头提取令牌.
func HeaderTokenExtractor(header string) TokenExtractor {
	return func(_ context.Context, request any) (string, error) {
		r, ok := request.(*http.Request)
		if !ok {
			return "", ErrMissingToken
		}
		val := r.Header.Get(header)
		if val == "" {
			return "", ErrMissingToken
		}
		return val, nil
	}
}

// QueryTokenExtractor 从 URL 查询参数提取令牌.
func QueryTokenExtractor(param string) TokenExtractor {
	return func(_ context.Context, request any) (string, error) {
		r, ok := request.(*http.Request)
		if !ok {
			return "", ErrMissingToken
		}
		val := r.URL.Query().Get(param)
		if val == "" {
			return "", ErrMissingToken
		}
		return val, nil
	}
}

// MetadataTokenExtractor 从 gRPC metadata 提取令牌.
func MetadataTokenExtractor(key string) TokenExtractor {
	return func(ctx context.Context, _ any) (string, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return "", ErrMissingToken
		}
		vals := md.Get(key)
		if len(vals) == 0 || vals[0] == "" {
			return "", ErrMissingToken
		}
		return vals[0], nil
	}
}

// PrincipalTokenExtractor 从 auth.Principal.ID 提取令牌（auth→tenant 桥接）.
func PrincipalTokenExtractor() TokenExtractor {
	return func(ctx context.Context, _ any) (string, error) {
		principal, ok := auth.FromContext(ctx)
		if !ok {
			return "", ErrMissingToken
		}
		if principal.ID == "" {
			return "", ErrMissingToken
		}
		return principal.ID, nil
	}
}
