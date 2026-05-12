package rbac

import (
	"context"

	"github.com/Tsukikage7/servex/v2/auth"
)

// Authorizer adapts an RBAC manager to auth.Authorizer.
type Authorizer struct {
	rbac RBAC
}

// NewAuthorizer creates an auth.Authorizer backed by RBAC.
func NewAuthorizer(rbac RBAC) *Authorizer {
	if rbac == nil {
		panic("auth/rbac: RBAC cannot be nil")
	}
	return &Authorizer{rbac: rbac}
}

// Authorize implements auth.Authorizer.
func (a *Authorizer) Authorize(ctx context.Context, principal *auth.Principal, target auth.Target) error {
	if principal == nil {
		return auth.ErrUnauthenticated
	}
	if a == nil || a.rbac == nil {
		return auth.ErrForbidden
	}
	if target.Resource == "" || target.Action == "" {
		return auth.ErrForbidden
	}

	ok, err := a.rbac.HasPermission(ctx, principal.ID, target.Resource, target.Action)
	if err != nil {
		return err
	}
	if !ok {
		return ErrPermissionDenied
	}
	return nil
}
