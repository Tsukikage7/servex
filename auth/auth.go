// Package auth 提供统一的认证授权框架.
//
// 特性:
//   - 可扩展的认证器接口
//   - 可插拔授权器接口
//   - HTTP/Endpoint 中间件
//   - gRPC 适配（auth/grpcx 子包）
//   - 内置 JWT 支持（auth/jwt 子包）
//
// 基本用法:
//
//	// 使用 JWT 认证
//	jwtAuth := jwt.NewAuthenticator(jwtSrv)
//	endpoint = auth.Middleware(jwtAuth)(endpoint)
//
//	// HTTP 中间件
//	handler = auth.HTTPMiddleware(jwtAuth)(handler)
//
//	// gRPC 拦截器
//	srv := grpc.NewServer(
//	    grpc.UnaryInterceptor(authgrpcx.UnaryServerInterceptor(jwtAuth)),
//	)
//
// 在业务逻辑中使用:
//
//	func CreateOrder(ctx context.Context, req *CreateOrderRequest) error {
//	    principal, ok := auth.FromContext(ctx)
//	    if !ok {
//	        return auth.ErrUnauthenticated
//	    }
//	    order.UserID = principal.ID
//	    return nil
//	}
package auth

import (
	"context"
	"time"
)

// Credentials 认证凭据.
type Credentials struct {
	// Type 凭据类型: bearer, api_key, basic.
	Type string

	// Token 凭据令牌.
	Token string

	// Extra 额外信息.
	Extra map[string]string
}

const (
	// CredentialTypeBearer Bearer 令牌凭据.
	CredentialTypeBearer = "bearer"
	// CredentialTypeAPIKey API Key 凭据.
	CredentialTypeAPIKey = "api_key"
	// CredentialTypeBasic Basic 认证凭据.
	CredentialTypeBasic = "basic"
)

// Principal 身份主体，表示已认证的用户/服务.
type Principal struct {
	// ID 唯一标识.
	ID string

	// Type 主体类型: user, service.
	Type string

	// Name 主体名称（可选）.
	Name string

	// Roles 角色列表.
	Roles []string

	// Permissions 权限列表.
	Permissions []string

	// Metadata 扩展元数据.
	Metadata map[string]any

	// ExpiresAt 过期时间.
	ExpiresAt *time.Time
}

const (
	// PrincipalTypeUser 用户类型主体.
	PrincipalTypeUser = "user"
	// PrincipalTypeService 服务类型主体.
	PrincipalTypeService = "service"
)

// HasRole 检查主体是否具有指定角色.
func (p *Principal) HasRole(role string) bool {
	for _, r := range p.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasPermission 检查主体是否具有指定权限.
func (p *Principal) HasPermission(permission string) bool {
	for _, perm := range p.Permissions {
		if perm == permission {
			return true
		}
	}
	return false
}

// HasAnyRole 检查主体是否具有任一指定角色.
func (p *Principal) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if p.HasRole(role) {
			return true
		}
	}
	return false
}

// HasAllRoles 检查主体是否具有所有指定角色.
func (p *Principal) HasAllRoles(roles ...string) bool {
	for _, role := range roles {
		if !p.HasRole(role) {
			return false
		}
	}
	return true
}

// IsExpired 检查主体是否已过期.
func (p *Principal) IsExpired() bool {
	if p.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*p.ExpiresAt)
}

// GetMetadata 获取元数据值.
func (p *Principal) GetMetadata(key string) (any, bool) {
	if p.Metadata == nil {
		return nil, false
	}
	v, ok := p.Metadata[key]
	return v, ok
}

// GetMetadataString 获取字符串类型的元数据值.
func (p *Principal) GetMetadataString(key string) string {
	v, ok := p.GetMetadata(key)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// Authenticator 认证器接口.
type Authenticator interface {
	// Authenticate 验证凭据，返回身份主体.
	Authenticate(ctx context.Context, creds Credentials) (*Principal, error)
}

// Authorizer 授权器接口.
type Authorizer interface {
	// Authorize 检查主体是否有权限执行操作.
	Authorize(ctx context.Context, principal *Principal, target Target) error
}

// CredentialsExtractor 凭据提取器函数.
type CredentialsExtractor func(ctx context.Context, request any) (*Credentials, error)

// Target 描述一次授权检查的目标.
type Target struct {
	// Resource 是业务资源名，如 "orders".
	Resource string

	// Action 是业务动作名，如 "read".
	Action string

	// Method 是 transport 方法名，如 HTTP method 或 gRPC full method.
	Method string

	// Path 是 HTTP path.
	Path string

	// Metadata 扩展授权上下文.
	Metadata map[string]string

	// Permissions 是声明式策略要求的权限列表.
	Permissions []string

	// AllPermissions 表示是否需要满足所有 Permissions.
	AllPermissions bool
}

// Skipper 跳过检查函数.
type Skipper func(ctx context.Context, request any) bool

// ErrorHandler 错误处理函数.
type ErrorHandler func(ctx context.Context, err error) error
