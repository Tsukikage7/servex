package grpcx

import (
	"context"
	"testing"
	"time"

	golangjwt "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	servexjwt "github.com/Tsukikage7/servex/v2/auth/jwt"
	"github.com/Tsukikage7/servex/v2/testx"
)

type testClaims struct {
	golangjwt.RegisteredClaims
	Username string `json:"username"`
}

func newTestJWT() *servexjwt.JWT {
	return servexjwt.MustNew(
		servexjwt.WithSecretKey("test-secret-key-for-testing-32b!"),
		servexjwt.WithLogger(testx.NopLogger()),
		servexjwt.WithIssuer("test-issuer"),
	)
}

func generateTestToken(t *testing.T, j *servexjwt.JWT, subject string) string {
	t.Helper()
	token, err := j.Generate(t.Context(), &servexjwt.StandardClaims{
		RegisteredClaims: golangjwt.RegisteredClaims{
			Subject:   subject,
			Issuer:    "test-issuer",
			ExpiresAt: golangjwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  golangjwt.NewNumericDate(time.Now()),
		},
	})
	require.NoError(t, err)
	return token
}

func TestUnaryServerInterceptor(t *testing.T) {
	j := newTestJWT()
	token := generateTestToken(t, j, "user-123")
	interceptor := UnaryServerInterceptor(j)

	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("authorization", "Bearer "+token))
	resp, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{FullMethod: "/svc.Auth/Check"}, func(ctx context.Context, req any) (any, error) {
		claims, ok := servexjwt.ClaimsFromContext(ctx)
		require.True(t, ok)
		subject, err := claims.GetSubject()
		require.NoError(t, err)
		assert.Equal(t, "user-123", subject)
		return "ok", nil
	})

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestUnaryServerInterceptor_InvalidToken(t *testing.T) {
	j := newTestJWT()
	interceptor := UnaryServerInterceptor(j)

	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("authorization", "Bearer invalid-token"))
	resp, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{FullMethod: "/svc.Auth/Check"}, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})

	assert.Nil(t, resp)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestUnaryServerInterceptorWithClaims(t *testing.T) {
	j := newTestJWT()
	token, err := j.Generate(t.Context(), &testClaims{
		RegisteredClaims: golangjwt.RegisteredClaims{
			Subject:   "user-456",
			Issuer:    "test-issuer",
			ExpiresAt: golangjwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  golangjwt.NewNumericDate(time.Now()),
		},
		Username: "alice",
	})
	require.NoError(t, err)

	interceptor := UnaryServerInterceptorWithClaims(j, func() servexjwt.Claims { return &testClaims{} })
	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("authorization", token))
	_, err = interceptor(ctx, "request", &grpc.UnaryServerInfo{FullMethod: "/svc.Auth/Check"}, func(ctx context.Context, req any) (any, error) {
		claims, ok := servexjwt.ClaimsFromContext(ctx)
		require.True(t, ok)
		custom, ok := claims.(*testClaims)
		require.True(t, ok)
		assert.Equal(t, "alice", custom.Username)
		return "ok", nil
	})
	require.NoError(t, err)
}

func TestExtractToken(t *testing.T) {
	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("authorization", "Bearer test-token"))
	token, err := ExtractToken(ctx)

	require.NoError(t, err)
	assert.Equal(t, "test-token", token)
}

func TestIsWhitelisted(t *testing.T) {
	t.Run("method via metadata", func(t *testing.T) {
		whitelist := servexjwt.NewWhitelist().AddGRPCMethods("/api.v1.Auth/")
		ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs(":path", "/api.v1.Auth/Login"))
		assert.True(t, IsWhitelisted(ctx, whitelist))
	})

	t.Run("internal service", func(t *testing.T) {
		whitelist := servexjwt.NewWhitelist().SetInternalServiceHeader("x-internal").SetInternalServiceSecret("service-a")
		ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("x-internal", "service-a"))
		assert.True(t, IsWhitelisted(ctx, whitelist))
	})
}
