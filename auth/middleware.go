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

	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (any, error) {
			authCtx, skipped, err := AuthenticateRequest(ctx, request, Target{}, authenticator, nil, opts...)
			if skipped {
				return next(ctx, request)
			}
			if err != nil {
				return nil, err
			}

			return next(authCtx, request)
		}
	}
}

// AuthenticateRequest 执行一次认证授权流程，供 transport 适配层复用.
func AuthenticateRequest(
	ctx context.Context,
	request any,
	target Target,
	authenticator Authenticator,
	defaultExtractor CredentialsExtractor,
	opts ...Option,
) (context.Context, bool, error) {
	if authenticator == nil {
		return ctx, false, ErrInvalidCredentials
	}

	o := defaultOptions(authenticator)
	for _, opt := range opts {
		opt(o)
	}
	if o.credentialsExtractor == nil {
		o.credentialsExtractor = defaultExtractor
	}

	if o.skipper != nil && o.skipper(ctx, request) {
		return ctx, true, nil
	}

	authCtx, _, err := authenticateAndAuthorize(ctx, request, o, target)
	if err != nil {
		return ctx, false, handleError(ctx, err, o)
	}
	return authCtx, false, nil
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
