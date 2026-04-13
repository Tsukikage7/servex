package rbac_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/auth/rbac"
)

func ExampleNewManager() {
	ctx := context.Background()
	store := rbac.NewMemoryStore()
	mgr := rbac.NewManager(store, rbac.WithSuperAdmin("superadmin"))

	// 创建角色.
	_ = mgr.CreateRole(ctx, &rbac.Role{
		Name:        "editor",
		Permissions: []string{"articles:read", "articles:write"},
	})

	// 分配角色.
	_ = mgr.AssignRole(ctx, "user-1", "editor")

	// 检查权限.
	ok, _ := mgr.HasPermission(ctx, "user-1", "articles", "read")
	fmt.Println("has articles:read:", ok)

	ok, _ = mgr.HasPermission(ctx, "user-1", "articles", "delete")
	fmt.Println("has articles:delete:", ok)
	// Output:
	// has articles:read: true
	// has articles:delete: false
}

func ExampleParsePermission() {
	p := rbac.ParsePermission("users:read")
	fmt.Println(p.Resource)
	fmt.Println(p.Action)
	fmt.Println(p.String())
	// Output:
	// users
	// read
	// users:read
}
