package apikey

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tsukikage7/servex/v2/auth"
)

func TestNew_NilValidator_Panic(t *testing.T) {
	assert.Panics(t, func() {
		New(nil)
	})
}

func TestAuthenticator_Authenticate_Success(t *testing.T) {
	principal := &auth.Principal{
		ID:   "user-123",
		Name: "Test User",
		Type: auth.PrincipalTypeUser,
	}

	authenticator := New(func(_ context.Context, key string) (*auth.Principal, error) {
		if key == "valid-key" {
			return principal, nil
		}
		return nil, auth.ErrInvalidCredentials
	})

	p, err := authenticator.Authenticate(t.Context(), auth.Credentials{
		Type:  auth.CredentialTypeAPIKey,
		Token: "valid-key",
	})
	require.NoError(t, err)
	assert.Equal(t, "user-123", p.ID)
}

func TestAuthenticator_Authenticate_EmptyType(t *testing.T) {
	// 空 Type 也允许
	authenticator := New(func(_ context.Context, key string) (*auth.Principal, error) {
		return &auth.Principal{ID: "svc-1"}, nil
	})

	p, err := authenticator.Authenticate(t.Context(), auth.Credentials{
		Token: "any-key",
	})
	require.NoError(t, err)
	assert.Equal(t, "svc-1", p.ID)
}

func TestAuthenticator_Authenticate_WrongType(t *testing.T) {
	authenticator := New(func(_ context.Context, key string) (*auth.Principal, error) {
		return &auth.Principal{ID: "x"}, nil
	})

	_, err := authenticator.Authenticate(t.Context(), auth.Credentials{
		Type:  auth.CredentialTypeBearer,
		Token: "some-token",
	})
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestAuthenticator_Authenticate_EmptyToken(t *testing.T) {
	authenticator := New(func(_ context.Context, key string) (*auth.Principal, error) {
		return nil, auth.ErrInvalidCredentials
	})

	_, err := authenticator.Authenticate(t.Context(), auth.Credentials{
		Type: auth.CredentialTypeAPIKey,
	})
	assert.ErrorIs(t, err, auth.ErrCredentialsNotFound)
}

func TestAuthenticator_Authenticate_InvalidKey(t *testing.T) {
	authenticator := New(func(_ context.Context, key string) (*auth.Principal, error) {
		return nil, auth.ErrInvalidCredentials
	})

	_, err := authenticator.Authenticate(t.Context(), auth.Credentials{
		Type:  auth.CredentialTypeAPIKey,
		Token: "invalid-key",
	})
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestAuthenticator_Authenticate_ExpiredPrincipal(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	authenticator := New(func(_ context.Context, key string) (*auth.Principal, error) {
		return &auth.Principal{
			ID:        "expired-user",
			ExpiresAt: &past,
		}, nil
	})

	_, err := authenticator.Authenticate(t.Context(), auth.Credentials{
		Type:  auth.CredentialTypeAPIKey,
		Token: "some-key",
	})
	assert.ErrorIs(t, err, auth.ErrCredentialsExpired)
}

func TestStaticValidator(t *testing.T) {
	principal := &auth.Principal{ID: "svc-a"}
	validator := StaticValidator(map[string]*auth.Principal{
		"key-a": principal,
	})

	t.Run("有效 Key", func(t *testing.T) {
		p, err := validator(t.Context(), "key-a")
		require.NoError(t, err)
		assert.Equal(t, "svc-a", p.ID)
	})

	t.Run("无效 Key", func(t *testing.T) {
		_, err := validator(t.Context(), "key-b")
		assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
	})
}
