// Package rbac 提供基于角色的访问控制（RBAC）模型.
//
// 特性：
//   - 角色管理（创建、删除、列表）
//   - 用户角色分配与撤销
//   - 权限检查（支持通配符 "*"）
//   - 角色继承（通过 ParentID）
//   - 超级管理员角色
//   - 可选缓存层
//   - HTTP 中间件
//
// 示例：
//
//	store := rbac.NewMemoryStore()
//	mgr := rbac.NewManager(store, rbac.WithSuperAdmin("superadmin"))
//
//	_ = mgr.CreateRole(ctx, &rbac.Role{
//	    Name:        "editor",
//	    Permissions: []string{"articles:read", "articles:write"},
//	})
//
//	_ = mgr.AssignRole(ctx, "user-1", "editor")
//	ok, _ := mgr.HasPermission(ctx, "user-1", "articles", "read") // true
package rbac

import (
	"context"
	stderrors "errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Tsukikage7/servex/v2/auth"
	"github.com/Tsukikage7/servex/v2/errors"
)

var (
	// ErrRoleNotFound 角色未找到错误.
	ErrRoleNotFound = errors.New(20301, "RBAC_ROLE_NOT_FOUND", "角色未找到").
		WithHTTP(404).WithGRPC(5) // codes.NotFound

	// ErrRoleExists 角色已存在错误.
	ErrRoleExists = errors.New(20302, "RBAC_ROLE_EXISTS", "角色已存在").
		WithHTTP(409).WithGRPC(6) // codes.AlreadyExists

	// ErrPermissionDenied 权限被拒绝错误.
	ErrPermissionDenied = errors.New(20303, "RBAC_PERMISSION_DENIED", "权限被拒绝").
		WithHTTP(403).WithGRPC(7) // codes.PermissionDenied
)

// Role 角色.
type Role struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"uniqueIndex"`
	Description string    `json:"description"`
	Permissions []string  `json:"permissions" gorm:"serializer:json"`
	ParentID    string    `json:"parent_id,omitempty"` // 角色继承
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// Permission 权限.
type Permission struct {
	Resource string `json:"resource"` // 如 "users", "orders"
	Action   string `json:"action"`   // 如 "read", "write", "delete", "*"
}

// String 返回权限的字符串表示，格式为 "resource:action".
func (p Permission) String() string {
	return p.Resource + ":" + p.Action
}

// ParsePermission 解析权限字符串.
//
// 格式: "resource:action"，如 "users:read".
func ParsePermission(s string) Permission {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 2 {
		return Permission{Resource: parts[0], Action: parts[1]}
	}
	return Permission{Resource: s, Action: "*"}
}

// RBAC 权限管理器接口.
type RBAC interface {
	// CreateRole 创建角色.
	CreateRole(ctx context.Context, role *Role) error
	// GetRole 获取角色.
	GetRole(ctx context.Context, name string) (*Role, error)
	// DeleteRole 删除角色.
	DeleteRole(ctx context.Context, name string) error
	// ListRoles 列出所有角色.
	ListRoles(ctx context.Context) ([]*Role, error)

	// AssignRole 为用户分配角色.
	AssignRole(ctx context.Context, userID string, roleName string) error
	// RevokeRole 撤销用户角色.
	RevokeRole(ctx context.Context, userID string, roleName string) error
	// GetUserRoles 获取用户的所有角色.
	GetUserRoles(ctx context.Context, userID string) ([]*Role, error)

	// HasPermission 检查用户是否拥有指定权限.
	HasPermission(ctx context.Context, userID string, resource, action string) (bool, error)
	// HasRole 检查用户是否拥有指定角色.
	HasRole(ctx context.Context, userID string, roleName string) (bool, error)
	// GetUserPermissions 获取用户的所有权限.
	GetUserPermissions(ctx context.Context, userID string) ([]string, error)
}

// Option 为 Manager 配置选项.
type Option func(*manager)

// WithCache 设置缓存函数.
//
// cache 函数签名: func(key string, ttl time.Duration, fn func() (any, error)) (any, error)
func WithCache(cache func(key string, ttl time.Duration, fn func() (any, error)) (any, error)) Option {
	return func(m *manager) {
		m.cache = cache
	}
}

// WithSuperAdmin 设置超级管理员角色名，拥有所有权限.
func WithSuperAdmin(role string) Option {
	return func(m *manager) {
		m.superAdmin = role
	}
}

// manager RBAC 管理器实现.
type manager struct {
	store      Store
	cache      func(key string, ttl time.Duration, fn func() (any, error)) (any, error)
	superAdmin string
}

// NewManager 创建 RBAC 管理器.
func NewManager(store Store, opts ...Option) RBAC {
	m := &manager{
		store: store,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *manager) CreateRole(ctx context.Context, role *Role) error {
	// 检查是否已存在
	existing, err := m.store.GetRole(ctx, role.Name)
	if err != nil && !stderrors.Is(err, ErrRoleNotFound) {
		return err
	}
	if existing != nil {
		return ErrRoleExists
	}
	if role.ID == "" {
		role.ID = role.Name
	}
	return m.store.SaveRole(ctx, role)
}

func (m *manager) GetRole(ctx context.Context, name string) (*Role, error) {
	return m.store.GetRole(ctx, name)
}

func (m *manager) DeleteRole(ctx context.Context, name string) error {
	return m.store.DeleteRole(ctx, name)
}

func (m *manager) ListRoles(ctx context.Context) ([]*Role, error) {
	return m.store.ListRoles(ctx)
}

func (m *manager) AssignRole(ctx context.Context, userID string, roleName string) error {
	// 检查角色是否存在
	_, err := m.store.GetRole(ctx, roleName)
	if err != nil {
		return err
	}
	return m.store.AssignRole(ctx, userID, roleName)
}

func (m *manager) RevokeRole(ctx context.Context, userID string, roleName string) error {
	return m.store.RevokeRole(ctx, userID, roleName)
}

func (m *manager) GetUserRoles(ctx context.Context, userID string) ([]*Role, error) {
	roleNames, err := m.store.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}

	var roles []*Role
	for _, name := range roleNames {
		role, err := m.store.GetRole(ctx, name)
		if err != nil {
			// 记录不存在或查询失败的角色，便于排查数据不一致问题
			slog.Warn("rbac: 获取用户角色失败", "user_id", userID, "role", name, "error", err)
			continue
		}
		roles = append(roles, role)
	}
	return roles, nil
}

// maxInheritanceDepth 角色继承链最大深度，防止循环引用或过深递归.
const maxInheritanceDepth = 10

// collectPermissions 收集角色及其父角色的所有权限.
func (m *manager) collectPermissions(ctx context.Context, role *Role, visited map[string]bool) []string {
	if visited[role.Name] {
		return nil // 避免循环
	}
	if len(visited) >= maxInheritanceDepth {
		return nil // 继承链深度超限，停止递归
	}
	visited[role.Name] = true

	perms := make([]string, len(role.Permissions))
	copy(perms, role.Permissions)

	// 收集父角色权限
	if role.ParentID != "" {
		parent, err := m.store.GetRole(ctx, role.ParentID)
		if err == nil {
			perms = append(perms, m.collectPermissions(ctx, parent, visited)...)
		}
	}

	return perms
}

func (m *manager) HasPermission(ctx context.Context, userID string, resource, action string) (bool, error) {
	roles, err := m.GetUserRoles(ctx, userID)
	if err != nil {
		return false, err
	}

	for _, role := range roles {
		// 超级管理员拥有所有权限
		if m.superAdmin != "" && role.Name == m.superAdmin {
			return true, nil
		}

		visited := make(map[string]bool)
		perms := m.collectPermissions(ctx, role, visited)
		for _, perm := range perms {
			p := ParsePermission(perm)
			if matchPermission(p, resource, action) {
				return true, nil
			}
		}
	}

	return false, nil
}

// matchPermission 检查权限是否匹配.
func matchPermission(p Permission, resource, action string) bool {
	// 资源匹配（支持通配符）
	if p.Resource != "*" && p.Resource != resource {
		return false
	}
	// 动作匹配（支持通配符）
	if p.Action != "*" && p.Action != action {
		return false
	}
	return true
}

func (m *manager) HasRole(ctx context.Context, userID string, roleName string) (bool, error) {
	roleNames, err := m.store.GetUserRoles(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, name := range roleNames {
		if name == roleName {
			return true, nil
		}
	}
	return false, nil
}

func (m *manager) GetUserPermissions(ctx context.Context, userID string) ([]string, error) {
	roles, err := m.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}

	permSet := make(map[string]bool)
	for _, role := range roles {
		visited := make(map[string]bool)
		perms := m.collectPermissions(ctx, role, visited)
		for _, p := range perms {
			permSet[p] = true
		}
	}

	result := make([]string, 0, len(permSet))
	for p := range permSet {
		result = append(result, p)
	}
	return result, nil
}

// HTTPMiddleware 返回 HTTP 中间件，检查请求者是否拥有指定资源和操作的权限.
//
// 从 auth.FromContext 获取 userID，检查权限.
func HTTPMiddleware(rbac RBAC, resource, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := auth.FromContext(r.Context())
			if !ok {
				http.Error(w, "未认证", http.StatusUnauthorized)
				return
			}

			has, err := rbac.HasPermission(r.Context(), principal.ID, resource, action)
			if err != nil {
				// 不暴露内部错误详情到客户端
				slog.Error("rbac: 权限检查错误", "user_id", principal.ID, "error", err)
				http.Error(w, "权限检查失败", http.StatusInternalServerError)
				return
			}
			if !has {
				http.Error(w, ErrPermissionDenied.Error(), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
