package auth

import (
	"context"

	"github.com/Tsukikage7/servex/v2/endpoint"
)

// Middleware 返回 Endpoint 认证中间件.
//
// 示例:
//
//	authenticator := jwt.NewAuthenticator(jwtSrv)
//	endpoint = auth.Middleware(authenticator)(endpoint)
func Middleware(authenticator Authenticator, opts ...Option) endpoint.Middleware {
	if authenticator == nil {
		panic("auth: 认证器不能为空")
	}

	o := defaultOptions(authenticator)
	for _, opt := range opts {
		opt(o)
	}

	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (any, error) {
			// 检查是否跳过
			if o.skipper != nil && o.skipper(ctx, request) {
				return next(ctx, request)
			}

			authCtx, _, err := authenticateAndAuthorize(ctx, request, o, Target{})
			if err != nil {
				return nil, handleError(ctx, err, o)
			}

			return next(authCtx, request)
		}
	}
}

// extractCredentials 提取凭据.
func extractCredentials(ctx context.Context, request any, o *options) (*Credentials, error) {
	if o.credentialsExtractor != nil {
		return o.credentialsExtractor(ctx, request)
	}
	if creds, ok := CredentialsFromContext(ctx); ok {
		return creds, nil
	}
	if credsProvider, ok := request.(interface{ Credentials() *Credentials }); ok {
		return credsProvider.Credentials(), nil
	}
	return nil, ErrCredentialsNotFound
}

// handleError 处理错误.
func handleError(ctx context.Context, err error, o *options) error {
	if o.errorHandler != nil {
		return o.errorHandler(ctx, err)
	}
	return err
}
