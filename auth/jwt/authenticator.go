package jwt

import (
	"context"
	"errors"
	"reflect"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Tsukikage7/servex/v2/auth"
)

// ClaimsMapper Claims 到 Principal 的映射函数.
type ClaimsMapper func(claims jwt.Claims) (*auth.Principal, error)

// Authenticator JWT 认证器，实现 auth.Authenticator 接口.
type Authenticator struct {
	jwt           *JWT
	claimsFactory ClaimsFactory
	claimsMapper  ClaimsMapper
}

// AuthenticatorOption 认证器选项.
type AuthenticatorOption func(*Authenticator)

// WithClaimsMapper 设置自定义 Claims 映射函数.
func WithClaimsMapper(mapper ClaimsMapper) AuthenticatorOption {
	return func(a *Authenticator) {
		if mapper != nil {
			a.claimsMapper = mapper
		}
	}
}

// WithClaimsFactory 设置自定义 Claims 工厂.
//
// 认证器每次验证都会调用 factory 创建新的 Claims 实例,避免请求之间复用同一对象.
func WithClaimsFactory(factory ClaimsFactory) AuthenticatorOption {
	return func(a *Authenticator) {
		if factory != nil {
			a.claimsFactory = factory
		}
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
		jwt:           jwtSrv,
		claimsFactory: func() Claims { return &StandardClaims{} },
		claimsMapper:  defaultClaimsMapper,
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

	claimsType := a.claimsFactory()
	if claimsType == nil {
		return nil, auth.ErrInvalidCredentials
	}

	// 验证 JWT
	claims, err := a.jwt.ValidateWithClaims(ctx, creds.Token, claimsType)
	if err != nil {
		if errors.Is(err, ErrTokenInvalid) {
			return nil, auth.ErrInvalidCredentials.WithCause(err)
		}
		return nil, err
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
func defaultClaimsMapper(claims jwt.Claims) (*auth.Principal, error) {
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
	if mapClaims, ok := claims.(jwt.MapClaims); ok {
		principal.Roles = append(principal.Roles, stringClaimSlice(mapClaims["roles"])...)
		principal.Roles = append(principal.Roles, stringClaimSlice(mapClaims["role"])...)
		principal.Permissions = append(principal.Permissions, stringClaimSlice(mapClaims["permissions"])...)

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

	if principal.Metadata["claims"] == nil {
		principal.Metadata["claims"] = claims
	}
	principal.Roles = append(principal.Roles, exportedStringSliceField(claims, "Roles")...)
	principal.Roles = append(principal.Roles, exportedStringSliceField(claims, "Role")...)
	principal.Permissions = append(principal.Permissions, exportedStringSliceField(claims, "Permissions")...)
	if principal.Name == "" {
		principal.Name = exportedStringField(claims, "Name", "Username")
	}
	if typ := exportedStringField(claims, "Type"); typ != "" {
		principal.Type = typ
	}

	if principal.ID == "" {
		return nil, auth.ErrInvalidCredentials
	}

	return principal, nil
}

func stringClaimSlice(v any) []string {
	switch x := v.(type) {
	case string:
		if x == "" {
			return nil
		}
		return []string{x}
	case []string:
		return append([]string(nil), x...)
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func exportedStringSliceField(claims jwt.Claims, names ...string) []string {
	v := reflect.Indirect(reflect.ValueOf(claims))
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return nil
	}
	for _, name := range names {
		field := v.FieldByName(name)
		if !field.IsValid() {
			continue
		}
		switch field.Kind() {
		case reflect.String:
			if field.String() == "" {
				return nil
			}
			return []string{field.String()}
		case reflect.Slice:
			if field.Type().Elem().Kind() != reflect.String {
				return nil
			}
			out := make([]string, 0, field.Len())
			for i := 0; i < field.Len(); i++ {
				if s := field.Index(i).String(); s != "" {
					out = append(out, s)
				}
			}
			return out
		}
	}
	return nil
}

func exportedStringField(claims jwt.Claims, names ...string) string {
	v := reflect.Indirect(reflect.ValueOf(claims))
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return ""
	}
	for _, name := range names {
		field := v.FieldByName(name)
		if field.IsValid() && field.Kind() == reflect.String {
			return field.String()
		}
	}
	return ""
}
