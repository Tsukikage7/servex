package auth

import (
	"context"

	"github.com/Tsukikage7/servex/endpoint"
	"github.com/Tsukikage7/servex/observability/logger"
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

			// 提取凭据
			creds, err := extractCredentials(ctx, request, o)
			if err != nil {
				if o.logger != nil {
					logger.FromContext(ctx).Debug("[Auth] 凭据提取失败", logger.Err(err))
				}
				return nil, handleError(ctx, ErrCredentialsNotFound, o)
			}

			// 认证
			principal, err := authenticator.Authenticate(ctx, *creds)
			if err != nil {
				if o.logger != nil {
					logger.FromContext(ctx).Warn("[Auth] 认证失败", logger.Err(err))
				}
				return nil, handleError(ctx, err, o)
			}

			// 检查主体是否已过期
			if principal.IsExpired() {
				if o.logger != nil {
					logger.FromContext(ctx).Warn("[Auth] 主体已过期",
						logger.String("principal_id", principal.ID),
					)
				}
				return nil, handleError(ctx, ErrCredentialsExpired, o)
			}

			// 将主体存入 context
			ctx = WithPrincipal(ctx, principal)

			// 授权（action/resource 从 options 获取，未配置时传空字符串由 authorizer 自行处理）
			if o.authorizer != nil {
				action := o.action
				resource := o.resource
				if err := o.authorizer.Authorize(ctx, principal, resource, action); err != nil {
					if o.logger != nil {
						logger.FromContext(ctx).Warn("[Auth] 授权失败",
							logger.String("principal_id", principal.ID),
							logger.Err(err),
						)
					}
					return nil, handleError(ctx, err, o)
				}
			}

			return next(ctx, request)
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

// RequireRoles 便捷函数，创建需要指定角色的中间件.
func RequireRoles(authenticator Authenticator, roles []string, opts ...Option) endpoint.Middleware {
	opts = append(opts, WithAuthorizer(NewRoleAuthorizer(roles)))
	return Middleware(authenticator, opts...)
}

// RequirePermissions 便捷函数，创建需要指定权限的中间件.
func RequirePermissions(authenticator Authenticator, permissions []string, opts ...Option) endpoint.Middleware {
	opts = append(opts, WithAuthorizer(NewPermissionAuthorizer(permissions)))
	return Middleware(authenticator, opts...)
}
