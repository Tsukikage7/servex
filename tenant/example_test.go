package tenant_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/tenant"
)

// exampleTenant 示例租户.
type exampleTenant struct {
	id      string
	enabled bool
}

func (t *exampleTenant) TenantID() string    { return t.id }
func (t *exampleTenant) TenantEnabled() bool { return t.enabled }

func ExampleWithTenant() {
	t := &exampleTenant{id: "tenant-001", enabled: true}
	ctx := tenant.WithTenant(context.Background(), t)

	got, ok := tenant.FromContext(ctx)
	fmt.Println(ok)
	fmt.Println(got.TenantID())
	fmt.Println(tenant.ID(ctx))
	// Output:
	// true
	// tenant-001
	// tenant-001
}
