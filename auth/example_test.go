package auth_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/auth"
)

func ExamplePrincipal() {
	p := &auth.Principal{
		ID:          "user-1",
		Type:        auth.PrincipalTypeUser,
		Name:        "Alice",
		Roles:       []string{"admin", "editor"},
		Permissions: []string{"articles:read", "articles:write"},
	}

	fmt.Println(p.HasRole("admin"))
	fmt.Println(p.HasRole("viewer"))
	fmt.Println(p.HasPermission("articles:read"))
	fmt.Println(p.HasAnyRole("viewer", "editor"))
	fmt.Println(p.HasAllRoles("admin", "editor"))
	fmt.Println(p.IsExpired())
	// Output:
	// true
	// false
	// true
	// true
	// true
	// false
}

func ExampleWithPrincipal() {
	p := &auth.Principal{
		ID:   "user-1",
		Type: auth.PrincipalTypeUser,
		Name: "Alice",
	}

	ctx := auth.WithPrincipal(context.Background(), p)
	got, ok := auth.FromContext(ctx)
	fmt.Println(ok)
	fmt.Println(got.ID)
	fmt.Println(got.Name)
	// Output:
	// true
	// user-1
	// Alice
}

func ExampleGetPrincipalID() {
	p := &auth.Principal{ID: "svc-42", Type: auth.PrincipalTypeService}
	ctx := auth.WithPrincipal(context.Background(), p)

	id, ok := auth.GetPrincipalID(ctx)
	fmt.Println(id, ok)
	// Output:
	// svc-42 true
}
