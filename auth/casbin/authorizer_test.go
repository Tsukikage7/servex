package casbin

import (
	"context"
	"errors"
	"testing"

	"github.com/Tsukikage7/servex/v2/auth"
)

type fakeEnforcer struct {
	allowed bool
	err     error
	got     []interface{}
}

func (f *fakeEnforcer) Enforce(rvals ...interface{}) (bool, error) {
	f.got = append([]interface{}(nil), rvals...)
	return f.allowed, f.err
}

func TestAuthorizer(t *testing.T) {
	t.Run("allowed", func(t *testing.T) {
		enforcer := &fakeEnforcer{allowed: true}
		authorizer := NewAuthorizer(enforcer)

		err := authorizer.Authorize(
			t.Context(),
			&auth.Principal{ID: "user-1"},
			auth.Target{Resource: "orders", Action: "read"},
		)
		if err != nil {
			t.Fatalf("Authorize() error = %v", err)
		}
		if len(enforcer.got) != 3 || enforcer.got[0] != "user-1" || enforcer.got[1] != "orders" || enforcer.got[2] != "read" {
			t.Fatalf("Enforce args = %#v", enforcer.got)
		}
	})

	t.Run("denied", func(t *testing.T) {
		authorizer := NewAuthorizer(&fakeEnforcer{})

		err := authorizer.Authorize(t.Context(), &auth.Principal{ID: "user-1"}, auth.Target{Resource: "orders", Action: "write"})
		if !errors.Is(err, auth.ErrForbidden) {
			t.Fatalf("Authorize() error = %v, want ErrForbidden", err)
		}
	})

	t.Run("nil principal", func(t *testing.T) {
		authorizer := NewAuthorizer(&fakeEnforcer{allowed: true})

		err := authorizer.Authorize(t.Context(), nil, auth.Target{Resource: "orders", Action: "read"})
		if !errors.Is(err, auth.ErrUnauthenticated) {
			t.Fatalf("Authorize() error = %v, want ErrUnauthenticated", err)
		}
	})

	t.Run("enforcer error", func(t *testing.T) {
		want := errors.New("enforce failed")
		authorizer := NewAuthorizer(&fakeEnforcer{err: want})

		err := authorizer.Authorize(t.Context(), &auth.Principal{ID: "user-1"}, auth.Target{Resource: "orders", Action: "read"})
		if !errors.Is(err, want) {
			t.Fatalf("Authorize() error = %v, want %v", err, want)
		}
	})
}

func TestWithRequestBuilder(t *testing.T) {
	enforcer := &fakeEnforcer{allowed: true}
	authorizer := NewAuthorizer(enforcer, WithRequestBuilder(func(_ context.Context, principal *auth.Principal, target auth.Target) []interface{} {
		return []interface{}{principal.ID, "tenant-1", target.Resource, target.Action}
	}))

	err := authorizer.Authorize(t.Context(), &auth.Principal{ID: "user-1"}, auth.Target{Resource: "orders", Action: "read"})
	if err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	if len(enforcer.got) != 4 || enforcer.got[1] != "tenant-1" {
		t.Fatalf("Enforce args = %#v", enforcer.got)
	}
}

func TestNewAuthorizer_PanicWithNilEnforcer(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewAuthorizer(nil) should panic")
		}
	}()
	NewAuthorizer(nil)
}
