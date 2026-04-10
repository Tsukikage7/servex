// Package oauth2 提供第三方 OAuth2 登录的统一抽象和多平台实现.
package oauth2

import (
	"context"
	"time"
)

// Token 表示从第三方获取的 OAuth2 令牌.
type Token struct {
	AccessToken  string
	TokenType    string
	RefreshToken string
	ExpiresAt    time.Time
	Scopes       []string
	Raw          map[string]any
}

// IsExpired 检查 token 是否已过期.
func (t *Token) IsExpired() bool {
	if t.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(t.ExpiresAt)
}

// UserInfo 表示从第三方获取的用户信息.
type UserInfo struct {
	ProviderID string
	Provider   string
	Name       string
	Email      string
	AvatarURL  string
	Extra      map[string]any
}

// Provider 对接一个第三方 OAuth2 平台.
type Provider interface {
	AuthURL(state string, opts ...AuthURLOption) string
	Exchange(ctx context.Context, code string) (*Token, error)
	Refresh(ctx context.Context, refreshToken string) (*Token, error)
	UserInfo(ctx context.Context, token *Token) (*UserInfo, error)
}

// ProviderWithState 支持携带 state 参数交换令牌的 Provider.
// 实现 PKCE 时需要 state 来查找对应的 code_verifier.
type ProviderWithState interface {
	Provider
	ExchangeWithState(ctx context.Context, code, state string) (*Token, error)
}

// StateStore 管理 OAuth2 state 参数，防 CSRF.
type StateStore interface {
	Generate(ctx context.Context) (string, error)
	Validate(ctx context.Context, state string) (bool, error)
}
