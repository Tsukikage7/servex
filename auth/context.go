package auth

import "context"

// contextKey 上下文键类型.
type contextKey string

const (
	principalContextKey   contextKey = "auth:principal"
	credentialsContextKey contextKey = "auth:credentials"
)

// WithPrincipal 将身份主体存入 context.
func WithPrincipal(ctx context.Context, principal *Principal) context.Context {
	return context.WithValue(ctx, principalContextKey, principal)
}

// FromContext 从 context 获取身份主体.
func FromContext(ctx context.Context) (*Principal, bool) {
	principal, ok := ctx.Value(principalContextKey).(*Principal)
	return principal, ok
}

// RequirePrincipal 从 context 获取身份主体，不存在时返回 ErrUnauthenticated。
func RequirePrincipal(ctx context.Context) (*Principal, error) {
	principal, ok := FromContext(ctx)
	if !ok || principal == nil {
		return nil, ErrUnauthenticated
	}
	return principal, nil
}

// MustFromContext 从 context 获取身份主体，不存在则 panic.
func MustFromContext(ctx context.Context) *Principal {
	principal, ok := FromContext(ctx)
	if !ok {
		panic("auth: 上下文中未找到主体")
	}
	return principal
}

// WithCredentials 将凭据存入 context.
func WithCredentials(ctx context.Context, creds *Credentials) context.Context {
	return context.WithValue(ctx, credentialsContextKey, creds)
}

// CredentialsFromContext 从 context 获取凭据.
func CredentialsFromContext(ctx context.Context) (*Credentials, bool) {
	creds, ok := ctx.Value(credentialsContextKey).(*Credentials)
	return creds, ok
}

// HasRole 检查当前 context 中的主体是否有指定角色.
func HasRole(ctx context.Context, role string) bool {
	principal, ok := FromContext(ctx)
	if !ok {
		return false
	}
	return principal.HasRole(role)
}

// HasPermission 检查当前 context 中的主体是否有指定权限.
func HasPermission(ctx context.Context, permission string) bool {
	principal, ok := FromContext(ctx)
	if !ok {
		return false
	}
	return principal.HasPermission(permission)
}

// GetPrincipalID 获取当前 context 中主体的 ID.
func GetPrincipalID(ctx context.Context) (string, bool) {
	principal, ok := FromContext(ctx)
	if !ok {
		return "", false
	}
	return principal.ID, true
}

// RequirePrincipalID 从 context 获取主体 ID，不存在时返回 ErrUnauthenticated。
func RequirePrincipalID(ctx context.Context) (string, error) {
	principal, err := RequirePrincipal(ctx)
	if err != nil {
		return "", err
	}
	if principal.ID == "" {
		return "", ErrUnauthenticated
	}
	return principal.ID, nil
}
