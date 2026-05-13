package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/endpoint"
)

func TestPrincipal_HasRole(t *testing.T) {
	tests := []struct {
		name      string
		principal *Principal
		role      string
		want      bool
	}{
		{
			name: "has role",
			principal: &Principal{
				Roles: []string{"admin", "user"},
			},
			role: "admin",
			want: true,
		},
		{
			name: "does not have role",
			principal: &Principal{
				Roles: []string{"user"},
			},
			role: "admin",
			want: false,
		},
		{
			name: "empty roles",
			principal: &Principal{
				Roles: []string{},
			},
			role: "admin",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.principal.HasRole(tt.role); got != tt.want {
				t.Errorf("Principal.HasRole() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrincipal_HasPermission(t *testing.T) {
	tests := []struct {
		name       string
		principal  *Principal
		permission string
		want       bool
	}{
		{
			name: "has permission",
			principal: &Principal{
				Permissions: []string{"read:orders", "write:orders"},
			},
			permission: "read:orders",
			want:       true,
		},
		{
			name: "does not have permission",
			principal: &Principal{
				Permissions: []string{"read:orders"},
			},
			permission: "write:orders",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.principal.HasPermission(tt.permission); got != tt.want {
				t.Errorf("Principal.HasPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrincipal_HasAnyRole(t *testing.T) {
	principal := &Principal{
		Roles: []string{"user", "editor"},
	}

	if !principal.HasAnyRole("admin", "user") {
		t.Error("should have any role")
	}

	if principal.HasAnyRole("admin", "superuser") {
		t.Error("should not have any role")
	}
}

func TestPrincipal_HasAllRoles(t *testing.T) {
	principal := &Principal{
		Roles: []string{"user", "editor", "admin"},
	}

	if !principal.HasAllRoles("user", "editor") {
		t.Error("should have all roles")
	}

	if principal.HasAllRoles("user", "superuser") {
		t.Error("should not have all roles")
	}
}

func TestPrincipal_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		principal *Principal
		want      bool
	}{
		{
			name:      "no expiry",
			principal: &Principal{},
			want:      false,
		},
		{
			name: "not expired",
			principal: &Principal{
				ExpiresAt: func() *time.Time {
					t := time.Now().Add(time.Hour)
					return &t
				}(),
			},
			want: false,
		},
		{
			name: "expired",
			principal: &Principal{
				ExpiresAt: func() *time.Time {
					t := time.Now().Add(-time.Hour)
					return &t
				}(),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.principal.IsExpired(); got != tt.want {
				t.Errorf("Principal.IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContext(t *testing.T) {
	ctx := t.Context()

	// 测试无 principal
	if _, ok := FromContext(ctx); ok {
		t.Error("should not have principal")
	}

	// 测试有 principal
	principal := &Principal{
		ID:    "user-123",
		Type:  PrincipalTypeUser,
		Roles: []string{"admin"},
	}
	ctx = WithPrincipal(ctx, principal)

	got, ok := FromContext(ctx)
	if !ok {
		t.Error("should have principal")
	}
	if got.ID != principal.ID {
		t.Errorf("got ID = %v, want %v", got.ID, principal.ID)
	}

	// 测试便捷函数
	if !HasRole(ctx, "admin") {
		t.Error("should have admin role")
	}
	if HasRole(ctx, "user") {
		t.Error("should not have user role")
	}

	id, ok := GetPrincipalID(ctx)
	if !ok || id != "user-123" {
		t.Errorf("GetPrincipalID() = %v, %v", id, ok)
	}
}

func TestMustFromContext_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("should panic")
		}
	}()

	ctx := t.Context()
	MustFromContext(ctx)
}

func TestMethodAuthorizer(t *testing.T) {
	ctx := t.Context()
	principal := &Principal{ID: "user-1"}
	var gotAction, gotResource string
	authorizer := NewMethodAuthorizer(MethodRule{
		Method:   "/svc.Admin/Create",
		Action:   "create",
		Resource: "admin",
		Authorizer: AuthorizerFunc(func(_ context.Context, p *Principal, target Target) error {
			if p == nil || p.ID != "user-1" {
				t.Fatalf("principal = %+v", p)
			}
			gotAction, gotResource = target.Action, target.Resource
			return nil
		}),
	})

	if err := authorizer.Authorize(ctx, principal, Target{Method: "/svc.Admin/Create"}); err != nil {
		t.Fatalf("Authorize 失败: %v", err)
	}
	if gotAction != "create" || gotResource != "admin" {
		t.Fatalf("授权目标 = %q/%q, want create/admin", gotAction, gotResource)
	}
	if err := authorizer.Authorize(ctx, principal, Target{Method: "/svc.Admin/List"}); err != nil {
		t.Fatalf("未配置方法应放行: %v", err)
	}
}

func TestMethodAuthorizerUsesTargetMethod(t *testing.T) {
	ctx := t.Context()
	principal := &Principal{ID: "user-1"}
	called := false
	authorizer := NewMethodAuthorizer(MethodRule{
		Method:   "/svc.Admin/Create",
		Action:   "create",
		Resource: "admin",
		Authorizer: AuthorizerFunc(func(_ context.Context, _ *Principal, target Target) error {
			called = true
			if target.Resource != "admin" || target.Action != "create" {
				t.Fatalf("授权目标 = %q/%q, want admin/create", target.Resource, target.Action)
			}
			return nil
		}),
	})

	if err := authorizer.Authorize(ctx, principal, Target{
		Method:   "/svc.Admin/Create",
		Resource: "proto-declared-resource",
	}); err != nil {
		t.Fatalf("Authorize 失败: %v", err)
	}
	if !called {
		t.Fatal("应按 target.Method 命中方法规则")
	}
}

func TestMiddleware_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("should panic with nil authenticator")
		}
	}()

	Middleware(nil)
}

func TestMiddleware_PassesAuthorizationTarget(t *testing.T) {
	authenticator := &mockAuthenticator{principal: &Principal{ID: "user-1"}}
	var gotAction, gotResource string
	var gotPrincipal *Principal
	var gotCredentials *Credentials
	middleware := Middleware(
		authenticator,
		WithCredentialsExtractor(func(context.Context, any) (*Credentials, error) {
			return &Credentials{Type: CredentialTypeBearer, Token: "token"}, nil
		}),
		WithAction("create"),
		WithResource("order"),
		WithAuthorizer(AuthorizerFunc(func(_ context.Context, _ *Principal, target Target) error {
			gotAction, gotResource = target.Action, target.Resource
			return nil
		})),
	)

	_, err := middleware(endpoint.Nop)(t.Context(), struct{}{})
	if err != nil {
		t.Fatalf("Middleware 返回错误: %v", err)
	}
	_, err = middleware(func(ctx context.Context, _ any) (any, error) {
		gotPrincipal, _ = FromContext(ctx)
		gotCredentials, _ = CredentialsFromContext(ctx)
		return struct{}{}, nil
	})(t.Context(), struct{}{})
	if err != nil {
		t.Fatalf("Middleware 返回错误: %v", err)
	}
	if gotAction != "create" || gotResource != "order" {
		t.Fatalf("授权目标 = %q/%q, want create/order", gotAction, gotResource)
	}
	if gotPrincipal == nil || gotPrincipal.ID != "user-1" {
		t.Fatalf("Principal 未写入 context: %+v", gotPrincipal)
	}
	if gotCredentials == nil || gotCredentials.Token != "token" {
		t.Fatalf("Credentials 未写入 context: %+v", gotCredentials)
	}
}

func TestIsUnauthenticated(t *testing.T) {
	if !IsUnauthenticated(ErrUnauthenticated) {
		t.Error("should be unauthenticated")
	}
	if IsUnauthenticated(ErrForbidden) {
		t.Error("should not be unauthenticated")
	}
}

func TestIsForbidden(t *testing.T) {
	if !IsForbidden(ErrForbidden) {
		t.Error("should be forbidden")
	}
	if IsForbidden(ErrUnauthenticated) {
		t.Error("should not be forbidden")
	}
}

func TestCredentialsFromContext(t *testing.T) {
	ctx := t.Context()

	// 无凭据
	if _, ok := CredentialsFromContext(ctx); ok {
		t.Error("should not have credentials")
	}

	// 有凭据
	creds := &Credentials{
		Type:  CredentialTypeBearer,
		Token: "test-token",
	}
	ctx = WithCredentials(ctx, creds)

	got, ok := CredentialsFromContext(ctx)
	if !ok {
		t.Error("should have credentials")
	}
	if got.Token != creds.Token {
		t.Errorf("got Token = %v, want %v", got.Token, creds.Token)
	}
}

func TestPrincipal_GetMetadata(t *testing.T) {
	principal := &Principal{
		Metadata: map[string]any{
			"key1": "value1",
			"key2": 123,
		},
	}

	// 获取存在的 key
	v, ok := principal.GetMetadata("key1")
	if !ok || v != "value1" {
		t.Errorf("GetMetadata(key1) = %v, %v", v, ok)
	}

	// 获取不存在的 key
	_, ok = principal.GetMetadata("key3")
	if ok {
		t.Error("should not have key3")
	}

	// GetMetadataString
	s := principal.GetMetadataString("key1")
	if s != "value1" {
		t.Errorf("GetMetadataString(key1) = %v", s)
	}

	s = principal.GetMetadataString("key2") // 非字符串类型
	if s != "" {
		t.Errorf("GetMetadataString(key2) should return empty, got %v", s)
	}

	// nil metadata
	p2 := &Principal{}
	_, ok = p2.GetMetadata("any")
	if ok {
		t.Error("should return false for nil metadata")
	}
}

// mockAuthenticator is a test authenticator.
type mockAuthenticator struct {
	principal *Principal
	err       error
}

func (m *mockAuthenticator) Authenticate(_ context.Context, creds Credentials) (*Principal, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.principal, nil
}

func TestHTTPMiddleware_Success(t *testing.T) {
	principal := &Principal{ID: "user-1", Roles: []string{"admin"}}
	auth := &mockAuthenticator{principal: principal}

	var gotPrincipal *Principal
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := FromContext(r.Context())
		if ok {
			gotPrincipal = p
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := HTTPMiddleware(auth)(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if gotPrincipal == nil || gotPrincipal.ID != "user-1" {
		t.Error("principal should be set in context")
	}
}

func TestHTTPMiddleware_NoCredentials(t *testing.T) {
	auth := &mockAuthenticator{principal: &Principal{ID: "user-1"}}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := HTTPMiddleware(auth)(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestHTTPMiddleware_AuthFails(t *testing.T) {
	auth := &mockAuthenticator{err: ErrInvalidCredentials}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := HTTPMiddleware(auth)(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestHTTPMiddleware_WithSkipper(t *testing.T) {
	auth := &mockAuthenticator{err: ErrInvalidCredentials}

	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := HTTPMiddleware(auth, WithSkipper(HTTPSkipPaths("/health", "/metrics")))(inner)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !called {
		t.Error("handler should be called for skipped path")
	}
}

func TestHTTPMiddleware_WithAuthorizer(t *testing.T) {
	principal := &Principal{ID: "user-1", Roles: []string{"user"}}
	auth := &mockAuthenticator{principal: principal}
	authorizer := AuthorizerFunc(func(context.Context, *Principal, Target) error {
		return ErrForbidden
	})

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := HTTPMiddleware(auth, WithAuthorizer(authorizer))(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/admin", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestHTTPMiddleware_PassesTarget(t *testing.T) {
	principal := &Principal{ID: "user-1"}
	auth := &mockAuthenticator{principal: principal}
	var got Target

	handler := HTTPMiddleware(auth, WithAuthorizer(AuthorizerFunc(func(_ context.Context, _ *Principal, target Target) error {
		got = target
		return nil
	})))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/orders", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got.Resource != "/api/orders" || got.Action != http.MethodPost || got.Path != "/api/orders" || got.Method != http.MethodPost {
		t.Fatalf("target = %+v", got)
	}
}

func TestHTTPMiddleware_WithExplicitTarget(t *testing.T) {
	principal := &Principal{ID: "user-1"}
	auth := &mockAuthenticator{principal: principal}
	var got Target

	handler := HTTPMiddleware(
		auth,
		WithTarget(Target{Resource: "orders", Action: "create"}),
		WithAuthorizer(AuthorizerFunc(func(_ context.Context, _ *Principal, target Target) error {
			got = target
			return nil
		})),
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/orders", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got.Resource != "orders" || got.Action != "create" || got.Path != "/api/orders" || got.Method != http.MethodPost {
		t.Fatalf("target = %+v", got)
	}
}

func TestHTTPMiddleware_ExpiredPrincipal(t *testing.T) {
	expiredAt := time.Now().Add(-time.Minute)
	auth := &mockAuthenticator{principal: &Principal{ID: "user-1", ExpiresAt: &expiredAt}}

	handler := HTTPMiddleware(auth)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer expired-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHTTPMiddleware_PanicWithNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("should panic with nil authenticator")
		}
	}()
	HTTPMiddleware(nil)
}

func TestDefaultHTTPCredentialsExtractor(t *testing.T) {
	t.Run("bearer token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer my-token")
		creds, err := DefaultHTTPCredentialsExtractor(t.Context(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if creds.Type != CredentialTypeBearer || creds.Token != "my-token" {
			t.Errorf("unexpected creds: %+v", creds)
		}
	})

	t.Run("api key header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-API-Key", "api-key-123")
		creds, err := DefaultHTTPCredentialsExtractor(t.Context(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if creds.Type != CredentialTypeAPIKey || creds.Token != "api-key-123" {
			t.Errorf("unexpected creds: %+v", creds)
		}
	})

	t.Run("query param no longer extracted by default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?access_token=query-token", nil)
		_, err := DefaultHTTPCredentialsExtractor(t.Context(), req)
		if err != ErrCredentialsNotFound {
			t.Errorf("expected ErrCredentialsNotFound, got %v", err)
		}
	})

	t.Run("query param via QueryParamExtractor", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?access_token=query-token", nil)
		creds, err := QueryParamExtractor(t.Context(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if creds.Type != CredentialTypeBearer || creds.Token != "query-token" {
			t.Errorf("unexpected creds: %+v", creds)
		}
	})

	t.Run("not http request", func(t *testing.T) {
		_, err := DefaultHTTPCredentialsExtractor(t.Context(), "not a request")
		if err != ErrCredentialsNotFound {
			t.Errorf("expected ErrCredentialsNotFound, got %v", err)
		}
	})

	t.Run("no credentials", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		_, err := DefaultHTTPCredentialsExtractor(t.Context(), req)
		if err != ErrCredentialsNotFound {
			t.Errorf("expected ErrCredentialsNotFound, got %v", err)
		}
	})
}

func TestBearerExtractor(t *testing.T) {
	t.Run("valid bearer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer my-token")
		creds, err := BearerExtractor(t.Context(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if creds.Token != "my-token" {
			t.Errorf("expected my-token, got %s", creds.Token)
		}
	})

	t.Run("no bearer prefix", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Basic abc123")
		_, err := BearerExtractor(t.Context(), req)
		if err != ErrCredentialsNotFound {
			t.Errorf("expected ErrCredentialsNotFound, got %v", err)
		}
	})

	t.Run("not http request", func(t *testing.T) {
		_, err := BearerExtractor(t.Context(), "not a request")
		if err != ErrCredentialsNotFound {
			t.Errorf("expected ErrCredentialsNotFound, got %v", err)
		}
	})
}

func TestHasPermission_Context(t *testing.T) {
	ctx := t.Context()

	// No principal
	if HasPermission(ctx, "read:orders") {
		t.Error("should not have permission without principal")
	}

	// With principal
	principal := &Principal{Permissions: []string{"read:orders"}}
	ctx = WithPrincipal(ctx, principal)

	if !HasPermission(ctx, "read:orders") {
		t.Error("should have read:orders permission")
	}
	if HasPermission(ctx, "write:orders") {
		t.Error("should not have write:orders permission")
	}
}

func TestGetPrincipalID_NoPrincipal(t *testing.T) {
	ctx := t.Context()
	_, ok := GetPrincipalID(ctx)
	if ok {
		t.Error("should not find principal ID")
	}
}

func TestHTTPSkipPaths(t *testing.T) {
	skipper := HTTPSkipPaths("/health", "/metrics")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	if !skipper(t.Context(), req) {
		t.Error("should skip /health")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/users", nil)
	if skipper(t.Context(), req) {
		t.Error("should not skip /api/users")
	}

	// Not an HTTP request
	if skipper(t.Context(), "not-a-request") {
		t.Error("should not skip non-HTTP request")
	}
}

func TestWithSkipper_ComposesSkippers(t *testing.T) {
	opts := defaultOptions(&mockAuthenticator{})

	WithSkipper(func(_ context.Context, request any) bool {
		return request == "first"
	})(opts)
	WithSkipper(func(_ context.Context, request any) bool {
		return request == "second"
	})(opts)

	if !opts.skipper(t.Context(), "first") {
		t.Fatal("first skipper should match")
	}
	if !opts.skipper(t.Context(), "second") {
		t.Fatal("second skipper should match")
	}
	if opts.skipper(t.Context(), "third") {
		t.Fatal("unknown request should not match")
	}
}

func TestEndpointMiddleware(t *testing.T) {
	t.Run("success with credentials in context", func(t *testing.T) {
		principal := &Principal{ID: "user-1", Roles: []string{"admin"}}
		auth := &mockAuthenticator{principal: principal}

		var gotPrincipal *Principal
		ep := func(ctx context.Context, req any) (any, error) {
			p, ok := FromContext(ctx)
			if ok {
				gotPrincipal = p
			}
			return "ok", nil
		}

		mw := Middleware(auth)
		wrapped := mw(ep)

		ctx := WithCredentials(t.Context(), &Credentials{Type: CredentialTypeBearer, Token: "test"})
		resp, err := wrapped(ctx, nil)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp != "ok" {
			t.Errorf("expected 'ok', got %v", resp)
		}
		if gotPrincipal == nil || gotPrincipal.ID != "user-1" {
			t.Error("principal should be set in context")
		}
	})

	t.Run("no credentials", func(t *testing.T) {
		auth := &mockAuthenticator{principal: &Principal{ID: "user-1"}}
		mw := Middleware(auth)
		wrapped := mw(func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		})

		_, err := wrapped(t.Context(), nil)
		if err == nil {
			t.Error("expected error for no credentials")
		}
	})

	t.Run("auth fails", func(t *testing.T) {
		auth := &mockAuthenticator{err: ErrInvalidCredentials}
		mw := Middleware(auth)
		wrapped := mw(func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		})

		ctx := WithCredentials(t.Context(), &Credentials{Type: CredentialTypeBearer, Token: "bad"})
		_, err := wrapped(ctx, nil)
		if err == nil {
			t.Error("expected error for invalid credentials")
		}
	})

	t.Run("with skipper", func(t *testing.T) {
		auth := &mockAuthenticator{err: ErrInvalidCredentials}
		skipper := func(_ context.Context, _ any) bool { return true }
		mw := Middleware(auth, WithSkipper(skipper))
		wrapped := mw(func(ctx context.Context, req any) (any, error) {
			return "skipped", nil
		})

		resp, err := wrapped(t.Context(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp != "skipped" {
			t.Errorf("expected 'skipped', got %v", resp)
		}
	})

	t.Run("with authorizer denied", func(t *testing.T) {
		principal := &Principal{ID: "user-1", Roles: []string{"user"}}
		auth := &mockAuthenticator{principal: principal}
		authorizer := AuthorizerFunc(func(context.Context, *Principal, Target) error {
			return ErrForbidden
		})

		mw := Middleware(auth, WithAuthorizer(authorizer))
		wrapped := mw(func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		})

		ctx := WithCredentials(t.Context(), &Credentials{Type: CredentialTypeBearer, Token: "test"})
		_, err := wrapped(ctx, nil)
		if err == nil {
			t.Error("expected error for unauthorized role")
		}
	})
}
