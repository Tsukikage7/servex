// Package casbin adapts a Casbin enforcer to auth.Authorizer.
package casbin

import (
	"context"

	"github.com/Tsukikage7/servex/v2/auth"
)

// Enforcer is the subset of Casbin's enforcer API required by this adapter.
//
// *casbin.Enforcer and *casbin.SyncedEnforcer both satisfy this interface.
type Enforcer interface {
	Enforce(rvals ...interface{}) (bool, error)
}

// RequestBuilder builds Casbin enforcement arguments.
//
// The default request is: subject, resource, action.
type RequestBuilder func(ctx context.Context, principal *auth.Principal, target auth.Target) []interface{}

// Option configures Authorizer.
type Option func(*Authorizer)

// WithRequestBuilder overrides how auth context maps to Casbin request values.
//
// Use this for domain-aware models, ABAC models, or custom subject selection.
func WithRequestBuilder(builder RequestBuilder) Option {
	return func(a *Authorizer) {
		if builder != nil {
			a.requestBuilder = builder
		}
	}
}

// Authorizer checks permissions with a Casbin-compatible enforcer.
type Authorizer struct {
	enforcer       Enforcer
	requestBuilder RequestBuilder
}

// NewAuthorizer creates a Casbin-backed auth.Authorizer.
func NewAuthorizer(enforcer Enforcer, opts ...Option) *Authorizer {
	if enforcer == nil {
		panic("auth/casbin: enforcer cannot be nil")
	}
	a := &Authorizer{
		enforcer:       enforcer,
		requestBuilder: defaultRequestBuilder,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Authorize implements auth.Authorizer.
func (a *Authorizer) Authorize(ctx context.Context, principal *auth.Principal, target auth.Target) error {
	if principal == nil {
		return auth.ErrUnauthenticated
	}
	if a == nil {
		return auth.ErrForbidden
	}

	allowed, err := a.enforcer.Enforce(a.requestBuilder(ctx, principal, target)...)
	if err != nil {
		return err
	}
	if !allowed {
		return auth.ErrForbidden
	}
	return nil
}

func defaultRequestBuilder(_ context.Context, principal *auth.Principal, target auth.Target) []interface{} {
	return []interface{}{principal.ID, target.Resource, target.Action}
}
