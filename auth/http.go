package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/Tsukikage7/servex/observability/logger"
)

// HTTPMiddleware 返回 HTTP 认证中间件.
//
// 默认从 Authorization 请求头提取 Bearer Token.
//
// 示例:
//
//	authenticator := jwt.NewAuthenticator(jwtSrv)
//	handler = auth.HTTPMiddleware(authenticator)(handler)
func HTTPMiddleware(authenticator Authenticator, opts ...Option) func(http.Handler) http.Handler {
	if authenticator == nil {
		panic("auth: 认证器不能为空")
	}

	o := defaultOptions(authenticator)
	for _, opt := range opts {
		opt(o)
	}

	if o.credentialsExtractor == nil {
		o.credentialsExtractor = DefaultHTTPCredentialsExtractor
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// 检查是否跳过
			if o.skipper != nil && o.skipper(ctx, r) {
				next.ServeHTTP(w, r)
				return
			}

			// 提取凭据
			creds, err := o.credentialsExtractor(ctx, r)
			if err != nil {
				if o.logger != nil {
					logger.FromContext(ctx).Debug("[Auth] HTTP凭据提取失败",
						logger.String("path", r.URL.Path),
						logger.Err(err),
					)
				}
				writeHTTPError(w, http.StatusUnauthorized, "Unauthorized")
				return
			}

			// 认证
			principal, err := authenticator.Authenticate(ctx, *creds)
			if err != nil {
				if o.logger != nil {
					logger.FromContext(ctx).Warn("[Auth] HTTP认证失败",
						logger.String("path", r.URL.Path),
						logger.Err(err),
					)
				}
				writeHTTPError(w, http.StatusUnauthorized, "Unauthorized")
				return
			}

			// 将主体存入 context
			ctx = WithPrincipal(ctx, principal)

			// 授权
			if o.authorizer != nil {
				if err := o.authorizer.Authorize(ctx, principal, "", r.URL.Path); err != nil {
					if o.logger != nil {
						logger.FromContext(ctx).Warn("[Auth] HTTP授权失败",
							logger.String("principal_id", principal.ID),
							logger.String("path", r.URL.Path),
							logger.Err(err),
						)
					}
					writeHTTPError(w, http.StatusForbidden, "Forbidden")
					return
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// DefaultHTTPCredentialsExtractor 默认的 HTTP 凭据提取器.
func DefaultHTTPCredentialsExtractor(_ context.Context, request any) (*Credentials, error) {
	r, ok := request.(*http.Request)
	if !ok {
		return nil, ErrCredentialsNotFound
	}

	// 1. Authorization Header (Bearer)
	if auth := r.Header.Get(AuthorizationHeader); auth != "" {
		if strings.HasPrefix(auth, BearerPrefix) {
			return &Credentials{
				Type:  CredentialTypeBearer,
				Token: strings.TrimPrefix(auth, BearerPrefix),
			}, nil
		}
	}

	// 2. X-API-Key Header
	if apiKey := r.Header.Get(APIKeyHeader); apiKey != "" {
		return &Credentials{
			Type:  CredentialTypeAPIKey,
			Token: apiKey,
		}, nil
	}

	// 3. Query 参数
	if token := r.URL.Query().Get("access_token"); token != "" {
		return &Credentials{
			Type:  CredentialTypeBearer,
			Token: token,
		}, nil
	}

	return nil, ErrCredentialsNotFound
}

// BearerExtractor 仅提取 Bearer Token.
func BearerExtractor(_ context.Context, request any) (*Credentials, error) {
	r, ok := request.(*http.Request)
	if !ok {
		return nil, ErrCredentialsNotFound
	}

	auth := r.Header.Get(AuthorizationHeader)
	if auth == "" || !strings.HasPrefix(auth, BearerPrefix) {
		return nil, ErrCredentialsNotFound
	}

	return &Credentials{
		Type:  CredentialTypeBearer,
		Token: strings.TrimPrefix(auth, BearerPrefix),
	}, nil
}

// writeHTTPError 写入 HTTP 错误响应.
func writeHTTPError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(`{"error":"` + message + `"}`))
}

// HTTPSkipPaths 返回跳过指定路径的 Skipper.
func HTTPSkipPaths(paths ...string) Skipper {
	pathSet := make(map[string]bool)
	for _, p := range paths {
		pathSet[p] = true
	}
	return func(_ context.Context, request any) bool {
		if r, ok := request.(*http.Request); ok {
			return pathSet[r.URL.Path]
		}
		return false
	}
}
