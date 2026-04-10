package tenant

import (
	"context"

	"github.com/Tsukikage7/servex/endpoint"
	"github.com/Tsukikage7/servex/observability/logger"
)

// Middleware 返回 Endpoint 租户解析中间件.
// 流程：skipper → 提取 token → resolve → 检查 enabled → WithTenant(ctx) → next.
// 示例:
//
//	endpoint = tenant.Middleware(resolver)(endpoint)
func Middleware(resolver Resolver, opts ...Option) endpoint.Middleware {
	if resolver == nil {
		panic("tenant: 解析器不能为空")
	}

	o := applyOptions(opts)

	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (any, error) {
			// 检查是否跳过
			if o.skipper != nil && o.skipper(ctx, request) {
				return next(ctx, request)
			}

			// 提取令牌
			token, err := extractToken(ctx, request, o)
			if err != nil {
				if o.logger != nil {
					logger.FromContext(ctx).Debug("[Tenant] 令牌提取失败", logger.Err(err))
				}
				return nil, handleError(ctx, ErrMissingToken, o)
			}

			// 解析租户
			t, err := resolver.Resolve(ctx, token)
			if err != nil {
				if o.logger != nil {
					// 日志中脱敏令牌，仅保留前 4 位，防止泄露完整令牌
					masked := maskToken(token)
					logger.FromContext(ctx).Warn("[Tenant] 解析失败",
						logger.String("token", masked),
						logger.Err(err),
					)
				}
				return nil, handleError(ctx, err, o)
			}

			// 检查租户是否启用
			if !t.TenantEnabled() {
				if o.logger != nil {
					logger.FromContext(ctx).Warn("[Tenant] 租户已禁用",
						logger.String("tenant_id", t.TenantID()),
					)
				}
				return nil, handleError(ctx, ErrTenantDisabled, o)
			}

			ctx = WithTenant(ctx, t)
			return next(ctx, request)
		}
	}
}

// extractToken 从请求中提取令牌.
func extractToken(ctx context.Context, request any, o *options) (string, error) {
	if o.tokenExtractor != nil {
		return o.tokenExtractor(ctx, request)
	}
	return "", ErrMissingToken
}

// maskToken 对令牌进行脱敏，仅保留前 4 个字符.
func maskToken(token string) string {
	if len(token) <= 4 {
		return "****"
	}
	return token[:4] + "****"
}
