package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
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

			target := Target{
				Resource: r.URL.Path,
				Action:   r.Method,
				Method:   r.Method,
				Path:     r.URL.Path,
			}
			authCtx, _, err := authenticateAndAuthorize(ctx, r, o, target)
			if err != nil {
				err = handleError(ctx, err, o)
				writeHTTPError(w, httpStatusForError(err), httpMessageForError(err))
				return
			}

			next.ServeHTTP(w, r.WithContext(authCtx))
		})
	}
}

// DefaultHTTPCredentialsExtractor 默认的 HTTP 凭据提取器.
//
// 仅从 Authorization Header（Bearer）和 X-API-Key Header 中提取凭据.
// 如需从 URL 查询参数 ?access_token= 提取，请使用 WithQueryParamExtraction() 显式启用.
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

	return nil, ErrCredentialsNotFound
}

// QueryParamExtractor 从 URL 查询参数 ?access_token= 中提取凭据.
//
// 注意: token 会出现在 URL、服务器日志和浏览器历史中，存在安全风险.
// 仅在 WebSocket 等无法设置 Header 的场景中使用.
func QueryParamExtractor(_ context.Context, request any) (*Credentials, error) {
	r, ok := request.(*http.Request)
	if !ok {
		return nil, ErrCredentialsNotFound
	}

	if token := r.URL.Query().Get("access_token"); token != "" {
		return &Credentials{
			Type:  CredentialTypeBearer,
			Token: token,
		}, nil
	}

	return nil, ErrCredentialsNotFound
}

// WithQueryParamExtraction 返回一个组合提取器，在默认提取器基础上额外支持 URL 查询参数 ?access_token=.
//
// 注意: token 会出现在 URL、服务器日志和浏览器历史中，存在安全风险.
// 仅在 WebSocket 等无法设置 Header 的场景中显式启用.
func WithQueryParamExtraction() Option {
	return WithCredentialsExtractor(func(ctx context.Context, request any) (*Credentials, error) {
		creds, err := DefaultHTTPCredentialsExtractor(ctx, request)
		if err == nil {
			return creds, nil
		}
		return QueryParamExtractor(ctx, request)
	})
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

// writeHTTPError 写入 HTTP 错误响应（使用 json.Marshal 防止 JSON 注入）.
func writeHTTPError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	body, _ := json.Marshal(map[string]string{"error": message})
	_, _ = w.Write(body)
}

func httpStatusForError(err error) int {
	if IsForbidden(err) {
		return http.StatusForbidden
	}
	return http.StatusUnauthorized
}

func httpMessageForError(err error) string {
	if IsForbidden(err) {
		return "Forbidden"
	}
	return "Unauthorized"
}

// HTTPSkipPaths 返回跳过指定路径的 Skipper.
// 同时匹配有无尾部斜杠的路径（如 /health 和 /health/）.
func HTTPSkipPaths(paths ...string) Skipper {
	pathSet := make(map[string]bool)
	for _, p := range paths {
		pathSet[strings.TrimRight(p, "/")] = true
	}
	return func(_ context.Context, request any) bool {
		if r, ok := request.(*http.Request); ok {
			return pathSet[strings.TrimRight(r.URL.Path, "/")]
		}
		return false
	}
}
