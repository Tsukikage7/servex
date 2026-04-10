// Package tenant 提供多租户支持，包括租户解析、context 传播、中间件和隔离工具.
package tenant

import (
	"context"
	"errors"
)

// ErrMissingTenant 上下文中未找到租户.
var ErrMissingTenant = errors.New("tenant: 上下文中未找到租户")

// Tenant 租户接口，应用层实现具体类型.
type Tenant interface {
	// TenantID 返回租户唯一标识.
	TenantID() string
	// TenantEnabled 返回租户是否启用.
	TenantEnabled() bool
}

// contextKey 上下文键类型.
type contextKey string

const tenantContextKey contextKey = "tenant:tenant"

// WithTenant 将租户存入 context.
func WithTenant(ctx context.Context, t Tenant) context.Context {
	return context.WithValue(ctx, tenantContextKey, t)
}

// FromContext 从 context 获取租户.
func FromContext(ctx context.Context) (Tenant, bool) {
	t, ok := ctx.Value(tenantContextKey).(Tenant)
	return t, ok
}

// FromContextErr 从 context 获取租户，不存在时返回 ErrMissingTenant.
// 适用于中间件或运行时配置场景，相比 MustFromContext 不会 panic.
func FromContextErr(ctx context.Context) (Tenant, error) {
	t, ok := FromContext(ctx)
	if !ok {
		return nil, ErrMissingTenant
	}
	return t, nil
}

// MustFromContext 从 context 获取租户，不存在则 panic.
func MustFromContext(ctx context.Context) Tenant {
	t, ok := FromContext(ctx)
	if !ok {
		panic("tenant: 上下文中未找到租户")
	}
	return t
}

// ID 从 context 获取租户 ID，无租户返回空字符串.
func ID(ctx context.Context) string {
	t, ok := FromContext(ctx)
	if !ok {
		return ""
	}
	return t.TenantID()
}
