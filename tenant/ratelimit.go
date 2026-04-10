package tenant

import (
	"context"
	"net/http"

	"github.com/Tsukikage7/servex/middleware/ratelimit"
)

// TenantHTTPKeyFunc 返回基于租户 ID 的 HTTP 限流键函数.
// 与 ratelimit.KeyedHTTPMiddleware 配合使用.
// 示例:
//
//	ratelimit.KeyedHTTPMiddleware(limiter, tenant.TenantHTTPKeyFunc())
func TenantHTTPKeyFunc() ratelimit.HTTPKeyFunc {
	return func(r *http.Request) string {
		return ID(r.Context())
	}
}

// TenantKeyFunc 返回基于租户 ID 的 Endpoint 限流键函数.
// 与 ratelimit.KeyedEndpointMiddleware 配合使用.
// 示例:
//
//	ratelimit.KeyedEndpointMiddleware(limiter, tenant.TenantKeyFunc())
func TenantKeyFunc() ratelimit.KeyFunc {
	return func(ctx context.Context, _ any) string {
		return ID(ctx)
	}
}
