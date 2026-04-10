package tenant

import (
	"testing"
)

// testTenant 测试用租户实现.
type testTenant struct {
	id      string
	enabled bool
}

func (t *testTenant) TenantID() string    { return t.id }
func (t *testTenant) TenantEnabled() bool { return t.enabled }

var _ Tenant = (*testTenant)(nil)

func TestWithTenantAndFromContext(t *testing.T) {
	ctx := t.Context()
	tn := &testTenant{id: "t1", enabled: true}

	ctx = WithTenant(ctx, tn)
	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext 应返回 true")
	}
	if got.TenantID() != "t1" {
		t.Fatalf("TenantID = %q, want %q", got.TenantID(), "t1")
	}
}

func TestFromContext_Empty(t *testing.T) {
	_, ok := FromContext(t.Context())
	if ok {
		t.Fatal("空 context 应返回 false")
	}
}

func TestMustFromContext(t *testing.T) {
	ctx := WithTenant(t.Context(), &testTenant{id: "t1", enabled: true})
	tn := MustFromContext(ctx)
	if tn.TenantID() != "t1" {
		t.Fatalf("TenantID = %q, want %q", tn.TenantID(), "t1")
	}
}

func TestMustFromContext_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustFromContext 应在无租户时 panic")
		}
	}()
	MustFromContext(t.Context())
}

func TestFromContextErr(t *testing.T) {
	ctx := WithTenant(t.Context(), &testTenant{id: "t1", enabled: true})
	tn, err := FromContextErr(ctx)
	if err != nil {
		t.Fatalf("FromContextErr 不应返回错误: %v", err)
	}
	if tn.TenantID() != "t1" {
		t.Fatalf("TenantID = %q, want %q", tn.TenantID(), "t1")
	}
}

func TestFromContextErr_Missing(t *testing.T) {
	_, err := FromContextErr(t.Context())
	if err == nil {
		t.Fatal("空 context 应返回错误")
	}
	if err != ErrMissingTenant {
		t.Fatalf("错误应为 ErrMissingTenant, got %v", err)
	}
}

func TestID(t *testing.T) {
	ctx := WithTenant(t.Context(), &testTenant{id: "abc", enabled: true})
	if got := ID(ctx); got != "abc" {
		t.Fatalf("ID = %q, want %q", got, "abc")
	}
}

func TestID_Empty(t *testing.T) {
	if got := ID(t.Context()); got != "" {
		t.Fatalf("ID = %q, want empty", got)
	}
}
