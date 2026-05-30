package jwt

import "context"

// contextKey 上下文键类型.
type contextKey string

const (
	// ClaimsContextKey Claims 上下文键.
	ClaimsContextKey contextKey = "jwt:claims"

	// TokenContextKey Token 上下文键.
	TokenContextKey contextKey = "jwt:token"
)

// ContextWithClaims 将 Claims 存入上下文.
func ContextWithClaims(ctx context.Context, claims Claims) context.Context {
	return context.WithValue(ctx, ClaimsContextKey, claims)
}

// ClaimsFromContext 从上下文获取 Claims.
func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(ClaimsContextKey).(Claims)
	return claims, ok
}

// ContextWithToken 将 Token 存入上下文.
func ContextWithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, TokenContextKey, token)
}

// TokenFromContext 从上下文获取 Token.
func TokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(TokenContextKey).(string)
	return token, ok
}

// GetSubjectFromContext 从上下文获取主题用户标识.
func GetSubjectFromContext(ctx context.Context) (string, bool) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return "", false
	}
	subject, err := claims.GetSubject()
	if err != nil {
		return "", false
	}
	return subject, true
}
