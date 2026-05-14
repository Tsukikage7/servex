package grpcx

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/Tsukikage7/servex/v2/auth"
)

func TestUnaryServerInterceptor_PassesTarget(t *testing.T) {
	authenticator := &mockAuthenticator{principal: &auth.Principal{ID: "user-1"}}
	var got auth.Target

	interceptor := UnaryServerInterceptor(authenticator, auth.WithAuthorizer(auth.AuthorizerFunc(func(_ context.Context, _ *auth.Principal, target auth.Target) error {
		got = target
		return nil
	})))

	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs(auth.GRPCAuthorizationMetadata, "Bearer valid-token"))
	_, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{FullMethod: "/svc.Order/Create"}, func(ctx context.Context, req any) (any, error) {
		if _, ok := auth.FromContext(ctx); !ok {
			t.Fatal("principal should be set in context")
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
	if got.Resource != "/svc.Order/Create" || got.Method != "/svc.Order/Create" {
		t.Fatalf("target = %+v", got)
	}
}

func TestUnaryServerInterceptor_ExpiredPrincipal(t *testing.T) {
	expiredAt := time.Now().Add(-time.Minute)
	authenticator := &mockAuthenticator{principal: &auth.Principal{ID: "user-1", ExpiresAt: &expiredAt}}
	interceptor := UnaryServerInterceptor(authenticator)

	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs(auth.GRPCAuthorizationMetadata, "Bearer expired-token"))
	_, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{FullMethod: "/svc.Order/Create"}, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})

	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("status.Code() = %v, want %v", status.Code(err), codes.Unauthenticated)
	}
}

func TestUnaryServerInterceptor_PolicyPermissionsOR(t *testing.T) {
	authenticator := &mockAuthenticator{principal: &auth.Principal{ID: "user-1"}}
	var checked []auth.Target

	interceptor := UnaryServerInterceptor(
		authenticator,
		auth.WithPolicyProvider(auth.MethodPolicyMap{
			"/svc.Order/Create": {
				FullMethod:  "/svc.Order/Create",
				Permissions: []string{"orders:read", "orders:create"},
			},
		}),
		auth.WithAuthorizer(auth.AuthorizerFunc(func(_ context.Context, _ *auth.Principal, target auth.Target) error {
			checked = append(checked, target)
			if target.Resource == "orders" && target.Action == "create" {
				return nil
			}
			return auth.ErrForbidden
		})),
	)

	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs(auth.GRPCAuthorizationMetadata, "Bearer valid-token"))
	_, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{FullMethod: "/svc.Order/Create"}, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
	if len(checked) != 2 {
		t.Fatalf("checked = %d, want 2", len(checked))
	}
	if checked[0].AllPermissions || checked[1].AllPermissions {
		t.Fatalf("policy metadata was not propagated: %+v", checked)
	}
	if len(checked[0].Permissions) != 2 || checked[0].Permissions[0] != "orders:read" {
		t.Fatalf("permissions were not propagated: %+v", checked[0])
	}
}

func TestUnaryServerInterceptor_PolicyPermissionsAND(t *testing.T) {
	authenticator := &mockAuthenticator{principal: &auth.Principal{ID: "user-1"}}
	var checked []auth.Target

	interceptor := UnaryServerInterceptor(
		authenticator,
		auth.WithPolicyProvider(auth.MethodPolicyMap{
			"/svc.Order/Ship": {
				FullMethod:     "/svc.Order/Ship",
				Permissions:    []string{"orders:write", "orders:ship"},
				AllPermissions: true,
			},
		}),
		auth.WithAuthorizer(auth.AuthorizerFunc(func(_ context.Context, _ *auth.Principal, target auth.Target) error {
			checked = append(checked, target)
			if target.Resource == "orders" && (target.Action == "write" || target.Action == "ship") {
				return nil
			}
			return auth.ErrForbidden
		})),
	)

	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs(auth.GRPCAuthorizationMetadata, "Bearer valid-token"))
	_, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{FullMethod: "/svc.Order/Ship"}, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
	if len(checked) != 2 {
		t.Fatalf("checked = %d, want 2", len(checked))
	}
}

func TestUnaryServerInterceptor_PolicyPermissionsRequireAuthorizer(t *testing.T) {
	authenticator := &mockAuthenticator{principal: &auth.Principal{ID: "user-1"}}

	interceptor := UnaryServerInterceptor(
		authenticator,
		auth.WithPolicyProvider(auth.MethodPolicyMap{
			"/svc.Order/Create": {
				FullMethod:  "/svc.Order/Create",
				Permissions: []string{"orders:create"},
			},
		}),
	)

	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs(auth.GRPCAuthorizationMetadata, "Bearer valid-token"))
	_, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{FullMethod: "/svc.Order/Create"}, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("status.Code() = %v, want %v", status.Code(err), codes.PermissionDenied)
	}
}

func TestUnaryServerInterceptor_EmptyPolicyKeepsConfiguredTarget(t *testing.T) {
	authenticator := &mockAuthenticator{principal: &auth.Principal{ID: "user-1"}}
	var got auth.Target

	interceptor := UnaryServerInterceptor(
		authenticator,
		auth.WithTarget(auth.Target{Resource: "orders", Action: "read"}),
		auth.WithPolicyProvider(auth.MethodPolicyMap{
			"/svc.Order/List": {
				FullMethod: "/svc.Order/List",
			},
		}),
		auth.WithAuthorizer(auth.AuthorizerFunc(func(_ context.Context, _ *auth.Principal, target auth.Target) error {
			got = target
			return nil
		})),
	)

	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs(auth.GRPCAuthorizationMetadata, "Bearer valid-token"))
	_, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{FullMethod: "/svc.Order/List"}, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
	if got.Resource != "orders" || got.Action != "read" {
		t.Fatalf("target = %+v, want configured resource/action", got)
	}
}

func TestUnaryServerInterceptor_PolicyPermissionsANDDenied(t *testing.T) {
	authenticator := &mockAuthenticator{principal: &auth.Principal{ID: "user-1"}}

	interceptor := UnaryServerInterceptor(
		authenticator,
		auth.WithPolicyProvider(auth.MethodPolicyMap{
			"/svc.Order/Ship": {
				FullMethod:     "/svc.Order/Ship",
				Permissions:    []string{"orders:write", "orders:ship"},
				AllPermissions: true,
			},
		}),
		auth.WithAuthorizer(auth.AuthorizerFunc(func(_ context.Context, _ *auth.Principal, target auth.Target) error {
			if target.Action == "write" {
				return nil
			}
			return auth.ErrForbidden
		})),
	)

	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs(auth.GRPCAuthorizationMetadata, "Bearer valid-token"))
	_, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{FullMethod: "/svc.Order/Ship"}, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("status.Code() = %v, want %v", status.Code(err), codes.PermissionDenied)
	}
}

type mockAuthenticator struct {
	principal *auth.Principal
	err       error
}

func (m *mockAuthenticator) Authenticate(_ context.Context, _ auth.Credentials) (*auth.Principal, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.principal, nil
}
