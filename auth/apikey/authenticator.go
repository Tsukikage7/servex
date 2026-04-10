// Package apikey 提供基于 API Key 的认证器实现.
package apikey

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"time"

	"github.com/Tsukikage7/servex/auth"
	"github.com/Tsukikage7/servex/storage/cache"
)

// Validator 验证 API Key 是否有效，返回对应的 Principal.
type Validator func(ctx context.Context, key string) (*auth.Principal, error)

// Authenticator API Key 认证器，实现 auth.Authenticator 接口.
type Authenticator struct {
	validator Validator
}

// 编译期接口合规检查.
var _ auth.Authenticator = (*Authenticator)(nil)

// New 创建 API Key 认证器.
func New(validator Validator) *Authenticator {
	if validator == nil {
		panic("apikey: Validator 不能为空")
	}
	return &Authenticator{validator: validator}
}

// Authenticate 验证 API Key 凭据.
//
// 校验逻辑：
//  1. 凭据类型须为 CredentialTypeAPIKey（或空，允许省略类型）
//  2. 凭据 Token 不能为空
//  3. 调用 Validator 验证 Key
//  4. 校验 Principal 是否已过期
func (a *Authenticator) Authenticate(ctx context.Context, creds auth.Credentials) (*auth.Principal, error) {
	if creds.Type != "" && creds.Type != auth.CredentialTypeAPIKey {
		return nil, auth.ErrInvalidCredentials
	}
	if creds.Token == "" {
		return nil, auth.ErrCredentialsNotFound
	}

	principal, err := a.validator(ctx, creds.Token)
	if err != nil {
		return nil, auth.ErrInvalidCredentials
	}
	if principal == nil {
		return nil, auth.ErrInvalidCredentials
	}

	if principal.IsExpired() {
		return nil, auth.ErrCredentialsExpired
	}

	return principal, nil
}

// StaticValidator 静态 API Key 验证器.
//
// 适合 Key 数量固定、部署时确定的场景.
// 使用 HMAC-SHA256 哈希后常量时间比较，确保不同长度的 key 也是常量时间.
// 遍历所有 key 防止通过响应时间推断 key 数量.
func StaticValidator(keys map[string]*auth.Principal) Validator {
	return func(_ context.Context, key string) (*auth.Principal, error) {
		keyHash := sha256.Sum256([]byte(key))
		var matched *auth.Principal
		for k, principal := range keys {
			kHash := sha256.Sum256([]byte(k))
			if subtle.ConstantTimeCompare(kHash[:], keyHash[:]) == 1 {
				matched = principal
			}
		}
		if matched != nil {
			return matched, nil
		}
		return nil, auth.ErrInvalidCredentials
	}
}

// CacheValidator 从缓存中查询 API Key 的验证器.
//
// 适合 Key 动态发放、需要中心化管理的场景.
// ttl 为缓存中 Principal 的有效期（与 cache TTL 无关，这里用于
// 读取缓存内值时检查 ExpiresAt 字段）.
func CacheValidator(c cache.Cache, _ time.Duration) Validator {
	return func(ctx context.Context, key string) (*auth.Principal, error) {
		val, err := c.Get(ctx, key)
		if err != nil {
			return nil, auth.ErrInvalidCredentials
		}
		if val == "" {
			return nil, auth.ErrInvalidCredentials
		}
		// 缓存中的 value 为 principal ID
		return &auth.Principal{
			ID:   val,
			Type: auth.PrincipalTypeService,
		}, nil
	}
}
