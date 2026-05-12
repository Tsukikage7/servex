package rbac

import (
	"errors"
	"testing"

	"github.com/Tsukikage7/servex/v2/auth"
)

func TestAuthorizer(t *testing.T) {
	ctx := t.Context()
	store := NewMemoryStore()
	manager := NewManager(store)

	if err := manager.CreateRole(ctx, &Role{
		Name:        "editor",
		Permissions: []string{"articles:write"},
	}); err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if err := manager.AssignRole(ctx, "user-1", "editor"); err != nil {
		t.Fatalf("AssignRole() error = %v", err)
	}

	authorizer := NewAuthorizer(manager)
	err := authorizer.Authorize(ctx, &auth.Principal{ID: "user-1"}, auth.Target{Resource: "articles", Action: "write"})
	if err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}

	err = authorizer.Authorize(ctx, &auth.Principal{ID: "user-1"}, auth.Target{Resource: "articles", Action: "delete"})
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Authorize() error = %v, want ErrPermissionDenied", err)
	}
}

func TestNewAuthorizer_PanicWithNilRBAC(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewAuthorizer(nil) should panic")
		}
	}()
	NewAuthorizer(nil)
}
