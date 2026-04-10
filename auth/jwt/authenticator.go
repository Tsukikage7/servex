package jwt

import (
	"context"

	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/Tsukikage7/servex/auth"
)

// ClaimsMapper Claims 到 Principal 的映射函数.
type ClaimsMapper func(claims gojwt.Claims) (*auth.Principal, error)

// Authenticator JWT 认证器，实现 auth.Authenticator 接口.
type Authenticator struct {
	jwt          *JWT
	claimsMapper ClaimsMapper
}

// AuthenticatorOption 认证器选项.
type AuthenticatorOption func(*Authenticator)

// WithClaimsMapper 设置自定义 Claims 映射函数.
func WithClaimsMapper(mapper ClaimsMapper) AuthenticatorOption {
	return func(a *Authenticator) {
		a.claimsMapper = mapper
	}
}

// NewAuthenticator 创建 JWT 认证器.
//
// 示例:
//
//	jwtSrv := jwt.NewJWT(jwt.WithSecretKey("secret"), jwt.WithLogger(log))
//	authenticator := jwt.NewAuthenticator(jwtSrv)
//
//	// 使用自定义 claims 映射
//	authenticator := jwt.NewAuthenticator(jwtSrv,
//	    jwt.WithClaimsMapper(func(claims jwt.Claims) (*auth.Principal, error) {
//	        // 自定义映射逻辑
//	    }),
//	)
func NewAuthenticator(jwtSrv *JWT, opts ...AuthenticatorOption) *Authenticator {
	if jwtSrv == nil {
		panic("jwt: JWT服务不能为空")
	}

	a := &Authenticator{
		jwt:          jwtSrv,
		claimsMapper: defaultClaimsMapper,
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

// Authenticate 实现 auth.Authenticator 接口.
func (a *Authenticator) Authenticate(ctx context.Context, creds auth.Credentials) (*auth.Principal, error) {
	if creds.Type != "" && creds.Type != auth.CredentialTypeBearer {
		return nil, auth.ErrInvalidCredentials
	}

	if creds.Token == "" {
		return nil, auth.ErrCredentialsNotFound
	}

	// 验证 JWT
	claims, err := a.jwt.Validate(ctx, creds.Token)
	if err != nil {
		return nil, auth.ErrInvalidCredentials
	}

	// 映射为 Principal
	principal, err := a.claimsMapper(claims)
	if err != nil {
		return nil, auth.ErrInvalidCredentials
	}

	// 检查过期
	if principal.IsExpired() {
		return nil, auth.ErrCredentialsExpired
	}

	return principal, nil
}

// defaultClaimsMapper 默认的 Claims 映射函数.
func defaultClaimsMapper(claims gojwt.Claims) (*auth.Principal, error) {
	principal := &auth.Principal{
		Type:     auth.PrincipalTypeUser,
		Metadata: make(map[string]any),
	}

	// 获取 subject 作为 ID
	if subject, err := claims.GetSubject(); err == nil {
		principal.ID = subject
	}

	// 获取过期时间
	if exp, err := claims.GetExpirationTime(); err == nil && exp != nil {
		principal.ExpiresAt = &exp.Time
	}

	// 尝试从 MapClaims 获取更多信息
	if mapClaims, ok := claims.(gojwt.MapClaims); ok {
		// 获取角色
		if roles, ok := mapClaims["roles"].([]any); ok {
			for _, r := range roles {
				if role, ok := r.(string); ok {
					principal.Roles = append(principal.Roles, role)
				}
			}
		}

		// 获取权限
		if permissions, ok := mapClaims["permissions"].([]any); ok {
			for _, p := range permissions {
				if perm, ok := p.(string); ok {
					principal.Permissions = append(principal.Permissions, perm)
				}
			}
		}

		// 获取名称
		if name, ok := mapClaims["name"].(string); ok {
			principal.Name = name
		}

		// 获取类型
		if typ, ok := mapClaims["type"].(string); ok {
			principal.Type = typ
		}

		// 保存完整的 claims 到 metadata
		principal.Metadata["claims"] = mapClaims
	}

	// 尝试从 StandardClaims 获取信息
	if stdClaims, ok := claims.(*StandardClaims); ok {
		if principal.ID == "" {
			principal.ID = stdClaims.Subject
		}
		if stdClaims.ExpiresAt != nil {
			principal.ExpiresAt = &stdClaims.ExpiresAt.Time
		}
	}

	if principal.ID == "" {
		return nil, auth.ErrInvalidCredentials
	}

	return principal, nil
}
