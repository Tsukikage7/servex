package rbac

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tsukikage7/servex/v2/auth"
)

func newTestManager(opts ...Option) (RBAC, Store) {
	store := NewMemoryStore()
	mgr := NewManager(store, opts...)
	return mgr, store
}

func TestCreateRole(t *testing.T) {
	mgr, _ := newTestManager()
	ctx := t.Context()

	role := &Role{
		Name:        "editor",
		Description: "文章编辑",
		Permissions: []string{"articles:read", "articles:write"},
	}

	err := mgr.CreateRole(ctx, role)
	require.NoError(t, err)

	// 重复创建
	err = mgr.CreateRole(ctx, role)
	assert.ErrorIs(t, err, ErrRoleExists)

	// 获取角色
	got, err := mgr.GetRole(ctx, "editor")
	require.NoError(t, err)
	assert.Equal(t, "editor", got.Name)
	assert.Equal(t, []string{"articles:read", "articles:write"}, got.Permissions)

	// 删除角色
	err = mgr.DeleteRole(ctx, "editor")
	require.NoError(t, err)

	_, err = mgr.GetRole(ctx, "editor")
	assert.ErrorIs(t, err, ErrRoleNotFound)
}

func TestAssignRole(t *testing.T) {
	mgr, _ := newTestManager()
	ctx := t.Context()

	// 先创建角色
	err := mgr.CreateRole(ctx, &Role{
		Name:        "viewer",
		Permissions: []string{"articles:read"},
	})
	require.NoError(t, err)

	// 分配角色
	err = mgr.AssignRole(ctx, "user-1", "viewer")
	require.NoError(t, err)

	// 检查角色
	has, err := mgr.HasRole(ctx, "user-1", "viewer")
	require.NoError(t, err)
	assert.True(t, has)

	has, err = mgr.HasRole(ctx, "user-1", "editor")
	require.NoError(t, err)
	assert.False(t, has)

	// 获取用户角色
	roles, err := mgr.GetUserRoles(ctx, "user-1")
	require.NoError(t, err)
	assert.Len(t, roles, 1)
	assert.Equal(t, "viewer", roles[0].Name)

	// 撤销角色
	err = mgr.RevokeRole(ctx, "user-1", "viewer")
	require.NoError(t, err)

	has, err = mgr.HasRole(ctx, "user-1", "viewer")
	require.NoError(t, err)
	assert.False(t, has)
}

func TestHasPermission(t *testing.T) {
	mgr, _ := newTestManager()
	ctx := t.Context()

	err := mgr.CreateRole(ctx, &Role{
		Name:        "editor",
		Permissions: []string{"articles:read", "articles:write"},
	})
	require.NoError(t, err)

	err = mgr.AssignRole(ctx, "user-1", "editor")
	require.NoError(t, err)

	// 有权限
	has, err := mgr.HasPermission(ctx, "user-1", "articles", "read")
	require.NoError(t, err)
	assert.True(t, has)

	has, err = mgr.HasPermission(ctx, "user-1", "articles", "write")
	require.NoError(t, err)
	assert.True(t, has)

	// 无权限
	has, err = mgr.HasPermission(ctx, "user-1", "articles", "delete")
	require.NoError(t, err)
	assert.False(t, has)

	has, err = mgr.HasPermission(ctx, "user-1", "users", "read")
	require.NoError(t, err)
	assert.False(t, has)

	// 通配符权限
	err = mgr.CreateRole(ctx, &Role{
		Name:        "admin",
		Permissions: []string{"*:*"},
	})
	require.NoError(t, err)
	err = mgr.AssignRole(ctx, "user-2", "admin")
	require.NoError(t, err)

	has, err = mgr.HasPermission(ctx, "user-2", "anything", "whatever")
	require.NoError(t, err)
	assert.True(t, has)
}

func TestRoleInheritance(t *testing.T) {
	mgr, _ := newTestManager()
	ctx := t.Context()

	// 创建父角色
	err := mgr.CreateRole(ctx, &Role{
		Name:        "viewer",
		Permissions: []string{"articles:read", "comments:read"},
	})
	require.NoError(t, err)

	// 创建子角色，继承 viewer
	err = mgr.CreateRole(ctx, &Role{
		Name:        "editor",
		ParentID:    "viewer",
		Permissions: []string{"articles:write"},
	})
	require.NoError(t, err)

	err = mgr.AssignRole(ctx, "user-1", "editor")
	require.NoError(t, err)

	// editor 自身权限
	has, err := mgr.HasPermission(ctx, "user-1", "articles", "write")
	require.NoError(t, err)
	assert.True(t, has)

	// 继承 viewer 的权限
	has, err = mgr.HasPermission(ctx, "user-1", "articles", "read")
	require.NoError(t, err)
	assert.True(t, has)

	has, err = mgr.HasPermission(ctx, "user-1", "comments", "read")
	require.NoError(t, err)
	assert.True(t, has)

	// 没有的权限
	has, err = mgr.HasPermission(ctx, "user-1", "comments", "write")
	require.NoError(t, err)
	assert.False(t, has)
}

func TestSuperAdmin(t *testing.T) {
	mgr, _ := newTestManager(WithSuperAdmin("superadmin"))
	ctx := t.Context()

	err := mgr.CreateRole(ctx, &Role{
		Name: "superadmin",
	})
	require.NoError(t, err)

	err = mgr.AssignRole(ctx, "admin-user", "superadmin")
	require.NoError(t, err)

	// 超级管理员拥有一切权限
	has, err := mgr.HasPermission(ctx, "admin-user", "anything", "whatever")
	require.NoError(t, err)
	assert.True(t, has)

	has, err = mgr.HasPermission(ctx, "admin-user", "secret", "delete")
	require.NoError(t, err)
	assert.True(t, has)
}

func TestHTTPMiddleware(t *testing.T) {
	mgr, _ := newTestManager()
	ctx := t.Context()

	err := mgr.CreateRole(ctx, &Role{
		Name:        "viewer",
		Permissions: []string{"articles:read"},
	})
	require.NoError(t, err)
	err = mgr.AssignRole(ctx, "user-1", "viewer")
	require.NoError(t, err)

	handler := HTTPMiddleware(mgr, "articles", "read")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	// 无认证信息 → 401
	req := httptest.NewRequest(http.MethodGet, "/articles", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// 有认证但无权限 → 403
	principal := &auth.Principal{ID: "user-nobody"}
	req = httptest.NewRequest(http.MethodGet, "/articles", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), principal))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// 有权限 → 200
	principal = &auth.Principal{ID: "user-1"}
	req = httptest.NewRequest(http.MethodGet, "/articles", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), principal))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestParsePermission(t *testing.T) {
	tests := []struct {
		input    string
		resource string
		action   string
	}{
		{"users:read", "users", "read"},
		{"orders:write", "orders", "write"},
		{"*:*", "*", "*"},
		{"users:*", "users", "*"},
		{"nocolon", "nocolon", "*"},
	}

	for _, tt := range tests {
		p := ParsePermission(tt.input)
		assert.Equal(t, tt.resource, p.Resource, "input: %s", tt.input)
		assert.Equal(t, tt.action, p.Action, "input: %s", tt.input)
		if tt.input != "nocolon" {
			assert.Equal(t, tt.input, p.String())
		}
	}
}

func TestListRoles(t *testing.T) {
	mgr, _ := newTestManager()
	ctx := t.Context()

	_ = mgr.CreateRole(ctx, &Role{Name: "a", Permissions: []string{"a:read"}})
	_ = mgr.CreateRole(ctx, &Role{Name: "b", Permissions: []string{"b:read"}})

	roles, err := mgr.ListRoles(ctx)
	require.NoError(t, err)
	assert.Len(t, roles, 2)
}

func TestGetUserPermissions(t *testing.T) {
	mgr, _ := newTestManager()
	ctx := t.Context()

	_ = mgr.CreateRole(ctx, &Role{Name: "r1", Permissions: []string{"a:read", "b:write"}})
	_ = mgr.CreateRole(ctx, &Role{Name: "r2", Permissions: []string{"c:read"}})
	_ = mgr.AssignRole(ctx, "u1", "r1")
	_ = mgr.AssignRole(ctx, "u1", "r2")

	perms, err := mgr.GetUserPermissions(ctx, "u1")
	require.NoError(t, err)
	assert.Len(t, perms, 3)
}
