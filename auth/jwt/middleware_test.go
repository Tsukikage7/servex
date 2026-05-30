package jwt

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tsukikage7/servex/v2/auth"
	"github.com/Tsukikage7/servex/v2/storage/cache"
	"github.com/Tsukikage7/servex/v2/testx"
)

// testClaims 测试用 Claims.
type testClaims struct {
	jwt.RegisteredClaims
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	Role     string   `json:"role"`
}

// newTestJWT 创建测试用 JWT 服务.
func newTestJWT() *JWT {
	return MustNew(
		WithSecretKey("test-secret-key-for-testing-32b!"),
		WithLogger(testx.NopLogger()),
		WithIssuer("test-issuer"),
	)
}

// generateTestToken 生成测试令牌.
func generateTestToken(j *JWT, subject string) string {
	claims := &StandardClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			Issuer:    "test-issuer",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token, _ := j.Generate(context.Background(), claims)
	return token
}

func TestAuthenticator_CustomClaimsFactory(t *testing.T) {
	j := newTestJWT()
	claims := &testClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			Issuer:    "test-issuer",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Username: "alice",
		Roles:    []string{"admin", "editor"},
	}
	token, err := j.Generate(t.Context(), claims)
	require.NoError(t, err)

	authenticator := NewAuthenticator(j,
		WithClaimsFactory(func() Claims { return &testClaims{} }),
	)
	principal, err := authenticator.Authenticate(t.Context(), authCredentials(token))
	require.NoError(t, err)

	assert.Equal(t, "user-123", principal.ID)
	assert.Equal(t, "alice", principal.Name)
	assert.ElementsMatch(t, []string{"admin", "editor"}, principal.Roles)
	parsed, ok := principal.Metadata["claims"].(*testClaims)
	require.True(t, ok)
	assert.Equal(t, "alice", parsed.Username)
}

func TestAuthenticator_CustomClaimsMapperReceivesCustomClaims(t *testing.T) {
	j := newTestJWT()
	token, err := j.Generate(t.Context(), &testClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-456",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Username: "bob",
		Roles:    []string{"operator"},
	})
	require.NoError(t, err)

	var gotClaims jwt.Claims
	authenticator := NewAuthenticator(j,
		WithClaimsFactory(func() Claims { return &testClaims{} }),
		WithClaimsMapper(func(claims jwt.Claims) (*auth.Principal, error) {
			gotClaims = claims
			c, ok := claims.(*testClaims)
			require.True(t, ok)
			return &auth.Principal{
				ID:    c.Subject,
				Type:  auth.PrincipalTypeUser,
				Name:  c.Username,
				Roles: c.Roles,
			}, nil
		}),
	)

	principal, err := authenticator.Authenticate(t.Context(), authCredentials(token))
	require.NoError(t, err)
	require.IsType(t, &testClaims{}, gotClaims)
	assert.Equal(t, "bob", principal.Name)
	assert.Equal(t, []string{"operator"}, principal.Roles)
}

func TestAuthenticator_DefaultMapperMapClaims(t *testing.T) {
	j := newTestJWT()
	claims := jwt.MapClaims{
		"sub":         "user-789",
		"exp":         time.Now().Add(time.Hour).Unix(),
		"iat":         time.Now().Unix(),
		"name":        "carol",
		"role":        "admin",
		"roles":       []string{"reviewer"},
		"permissions": []any{"orders:read", "orders:write"},
	}
	token, err := j.Generate(t.Context(), claims)
	require.NoError(t, err)

	authenticator := NewAuthenticator(j,
		WithClaimsFactory(func() Claims { return jwt.MapClaims{} }),
	)
	principal, err := authenticator.Authenticate(t.Context(), authCredentials(token))
	require.NoError(t, err)

	assert.Equal(t, "user-789", principal.ID)
	assert.Equal(t, "carol", principal.Name)
	assert.ElementsMatch(t, []string{"admin", "reviewer"}, principal.Roles)
	assert.ElementsMatch(t, []string{"orders:read", "orders:write"}, principal.Permissions)
}

func authCredentials(token string) auth.Credentials {
	return auth.Credentials{Type: auth.CredentialTypeBearer, Token: token}
}

type failingTokenStore struct {
	err error
}

func (s *failingTokenStore) Get(context.Context, string) (string, error) {
	return "", s.err
}

func (s *failingTokenStore) Set(context.Context, string, string, time.Duration) error {
	return nil
}

func (s *failingTokenStore) Delete(context.Context, ...string) error {
	return nil
}

func TestNewSigner(t *testing.T) {
	j := newTestJWT()

	t.Run("成功签名", func(t *testing.T) {
		claims := &StandardClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "user-123",
				Issuer:    "test-issuer",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}

		var capturedToken string
		endpoint := func(ctx context.Context, req any) (any, error) {
			// 验证 context 中有 Token
			token, ok := TokenFromContext(ctx)
			assert.True(t, ok)
			assert.NotEmpty(t, token)
			capturedToken = token
			return "success", nil
		}

		middleware := NewSigner(j)
		wrapped := middleware(endpoint)

		// 创建带 Claims 的 context
		ctx := ContextWithClaims(t.Context(), claims)

		resp, err := wrapped(ctx, nil)

		assert.NoError(t, err)
		assert.Equal(t, "success", resp)
		assert.NotEmpty(t, capturedToken)

		// 验证生成的令牌可以被解析
		validatedClaims, err := j.Validate(t.Context(), capturedToken)
		assert.NoError(t, err)
		subject, _ := validatedClaims.GetSubject()
		assert.Equal(t, "user-123", subject)
	})

	t.Run("无 Claims 时跳过签名", func(t *testing.T) {
		endpoint := func(ctx context.Context, req any) (any, error) {
			// 验证 context 中没有 Token
			_, ok := TokenFromContext(ctx)
			assert.False(t, ok)
			return "success", nil
		}

		middleware := NewSigner(j)
		wrapped := middleware(endpoint)

		resp, err := wrapped(t.Context(), nil)

		assert.NoError(t, err)
		assert.Equal(t, "success", resp)
	})
}

func TestNewParser(t *testing.T) {
	j := newTestJWT()

	t.Run("成功验证", func(t *testing.T) {
		token := generateTestToken(j, "user-123")

		endpoint := func(ctx context.Context, req any) (any, error) {
			// 验证 context 中有 Claims
			claims, ok := ClaimsFromContext(ctx)
			assert.True(t, ok)
			assert.NotNil(t, claims)

			// 验证 context 中有 Token
			ctxToken, ok := TokenFromContext(ctx)
			assert.True(t, ok)
			assert.NotEmpty(t, ctxToken)

			return "success", nil
		}

		middleware := NewParser(j)
		wrapped := middleware(endpoint)

		// 创建带 token 的 context
		ctx := ContextWithToken(t.Context(), token)

		resp, err := wrapped(ctx, nil)

		assert.NoError(t, err)
		assert.Equal(t, "success", resp)
	})

	t.Run("缺少令牌", func(t *testing.T) {
		endpoint := func(ctx context.Context, req any) (any, error) {
			return "success", nil
		}

		middleware := NewParser(j)
		wrapped := middleware(endpoint)

		resp, err := wrapped(t.Context(), nil)

		assert.Error(t, err)
		assert.Equal(t, ErrTokenNotFound, err)
		assert.Nil(t, resp)
	})

	t.Run("无效令牌", func(t *testing.T) {
		endpoint := func(ctx context.Context, req any) (any, error) {
			return "success", nil
		}

		middleware := NewParser(j)
		wrapped := middleware(endpoint)

		ctx := ContextWithToken(t.Context(), "invalid-token")

		resp, err := wrapped(ctx, nil)

		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("过期令牌", func(t *testing.T) {
		// 创建过期令牌
		claims := &StandardClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "user-123",
				Issuer:    "test-issuer",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // 已过期
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			},
		}
		token, _ := j.Generate(t.Context(), claims)

		endpoint := func(ctx context.Context, req any) (any, error) {
			return "success", nil
		}

		middleware := NewParser(j)
		wrapped := middleware(endpoint)

		ctx := ContextWithToken(t.Context(), token)

		resp, err := wrapped(ctx, nil)

		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestNewParserWithClaims(t *testing.T) {
	j := newTestJWT()

	t.Run("自定义 Claims 类型", func(t *testing.T) {
		// 生成带自定义 Claims 的令牌
		customClaims := &testClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "user-456",
				Issuer:    "test-issuer",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
			UserID:   "456",
			Username: "testuser",
		}
		token, err := j.Generate(t.Context(), customClaims)
		require.NoError(t, err)

		endpoint := func(ctx context.Context, req any) (any, error) {
			claims, ok := ClaimsFromContext(ctx)
			assert.True(t, ok)
			assert.NotNil(t, claims)
			return "success", nil
		}

		claimsFactory := func() Claims {
			return &testClaims{}
		}

		middleware := NewParserWithClaims(j, claimsFactory)
		wrapped := middleware(endpoint)

		ctx := ContextWithToken(t.Context(), token)

		resp, err := wrapped(ctx, nil)

		assert.NoError(t, err)
		assert.Equal(t, "success", resp)
	})
}

func TestNewParser_Whitelist(t *testing.T) {
	whitelist := NewWhitelist()
	whitelist.AddHTTPPaths("/health", "/metrics")

	j := MustNew(
		WithSecretKey("test-secret-key-at-least-32bytes!"),
		WithLogger(testx.NopLogger()),
		WithWhitelist(whitelist),
	)

	t.Run("白名单路径跳过验证", func(t *testing.T) {
		endpoint := func(ctx context.Context, req any) (any, error) {
			// 白名单请求不应有 Claims
			_, ok := ClaimsFromContext(ctx)
			assert.False(t, ok)
			return "success", nil
		}

		middleware := NewParser(j)
		wrapped := middleware(endpoint)

		// 模拟白名单请求通过 HTTP 请求
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		resp, err := wrapped(t.Context(), req)

		assert.NoError(t, err)
		assert.Equal(t, "success", resp)
	})
}

func TestNewParser_ContextPropagation(t *testing.T) {
	j := newTestJWT()
	token := generateTestToken(j, "user-789")

	t.Run("Claims 正确传播到下游", func(t *testing.T) {
		var capturedClaims Claims

		endpoint := func(ctx context.Context, req any) (any, error) {
			claims, ok := ClaimsFromContext(ctx)
			if ok {
				capturedClaims = claims
			}
			return "success", nil
		}

		middleware := NewParser(j)
		wrapped := middleware(endpoint)

		ctx := ContextWithToken(t.Context(), token)

		resp, err := wrapped(ctx, nil)

		assert.NoError(t, err)
		assert.Equal(t, "success", resp)
		assert.NotNil(t, capturedClaims)

		subject, err := capturedClaims.GetSubject()
		assert.NoError(t, err)
		assert.Equal(t, "user-789", subject)
	})

	t.Run("Token 正确传播到下游", func(t *testing.T) {
		var capturedToken string

		endpoint := func(ctx context.Context, req any) (any, error) {
			if t, ok := TokenFromContext(ctx); ok {
				capturedToken = t
			}
			return "success", nil
		}

		middleware := NewParser(j)
		wrapped := middleware(endpoint)

		ctx := ContextWithToken(t.Context(), token)

		resp, err := wrapped(ctx, nil)

		assert.NoError(t, err)
		assert.Equal(t, "success", resp)
		assert.NotEmpty(t, capturedToken)
	})
}

func TestNewParser_Concurrent(t *testing.T) {
	j := newTestJWT()

	endpoint := func(ctx context.Context, req any) (any, error) {
		claims, ok := ClaimsFromContext(ctx)
		assert.True(t, ok)
		assert.NotNil(t, claims)
		return "ok", nil
	}

	middleware := NewParser(j)
	wrapped := middleware(endpoint)

	// 并发调用
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(id int) {
			token := generateTestToken(j, "user-"+string(rune('a'+id%26)))
			ctx := ContextWithToken(t.Context(), token)

			resp, err := wrapped(ctx, nil)
			assert.NoError(t, err)
			assert.Equal(t, "ok", resp)
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestHTTPMiddleware(t *testing.T) {
	j := newTestJWT()

	t.Run("成功验证", func(t *testing.T) {
		token := generateTestToken(j, "user-123")

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 验证 context 中有 Claims
			claims, ok := ClaimsFromContext(r.Context())
			assert.True(t, ok)
			assert.NotNil(t, claims)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		wrapped := HTTPMiddleware(j)(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.Header.Set("Authorization", token)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "OK", rec.Body.String())
	})

	t.Run("缺少令牌", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := HTTPMiddleware(j)(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("无效令牌", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := HTTPMiddleware(j)(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestHTTPMiddleware_Whitelist(t *testing.T) {
	whitelist := NewWhitelist()
	whitelist.AddHTTPPaths("/health", "/public/")

	j := MustNew(
		WithSecretKey("test-secret-key-at-least-32bytes!"),
		WithLogger(testx.NopLogger()),
		WithWhitelist(whitelist),
	)

	t.Run("精确匹配白名单", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		wrapped := HTTPMiddleware(j)(handler)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("前缀匹配白名单", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := HTTPMiddleware(j)(handler)

		req := httptest.NewRequest(http.MethodGet, "/public/images/logo.png", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestExtractToken(t *testing.T) {
	t.Run("从 HTTP 请求提取", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer http-token")

		token, err := ExtractToken(t.Context(), req)

		assert.NoError(t, err)
		assert.Equal(t, "http-token", token)
	})

	t.Run("从上下文提取", func(t *testing.T) {
		ctx := ContextWithToken(t.Context(), "context-token")

		token, err := ExtractToken(ctx, nil)

		assert.NoError(t, err)
		assert.Equal(t, "context-token", token)
	})

	t.Run("未找到令牌", func(t *testing.T) {
		token, err := ExtractToken(t.Context(), nil)

		assert.Error(t, err)
		assert.Equal(t, ErrTokenNotFound, err)
		assert.Empty(t, token)
	})
}

func TestExtractTokenFromHeader(t *testing.T) {
	testCases := []struct {
		name     string
		header   string
		expected string
	}{
		{"带 Bearer 前缀", "Bearer token123", "token123"},
		{"带小写 bearer 前缀", "bearer token123", "token123"},
		{"无前缀", "token123", "token123"},
		{"带空格", "Bearer   token123  ", "token123"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractTokenFromHeader(tc.header)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestJWT_GenerateValidateRefresh(t *testing.T) {
	j := newTestJWT()

	t.Run("generate and validate", func(t *testing.T) {
		claims := &StandardClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "user-100",
				Issuer:    "test-issuer",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}

		token, err := j.Generate(t.Context(), claims)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		validated, err := j.Validate(t.Context(), token)
		require.NoError(t, err)
		sub, _ := validated.GetSubject()
		assert.Equal(t, "user-100", sub)
	})

	t.Run("validate empty token", func(t *testing.T) {
		_, err := j.Validate(t.Context(), "")
		assert.ErrorIs(t, err, ErrTokenEmpty)
	})

	t.Run("validate invalid token", func(t *testing.T) {
		_, err := j.Validate(t.Context(), "Bearer invalid.token.here")
		assert.Error(t, err)
	})

	t.Run("refresh valid token", func(t *testing.T) {
		oldClaims := &StandardClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "user-200",
				Issuer:    "test-issuer",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}
		token, err := j.Generate(t.Context(), oldClaims)
		require.NoError(t, err)

		newClaims := &StandardClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "user-200",
				Issuer:    "test-issuer",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}
		newToken, err := j.Refresh(t.Context(), token, newClaims)
		require.NoError(t, err)
		assert.NotEmpty(t, newToken)
		assert.NotEqual(t, token, newToken)
	})
}

func TestJWT_GenerateWithDuration(t *testing.T) {
	j := newTestJWT()

	t.Run("standard claims", func(t *testing.T) {
		claims := &StandardClaims{
			RegisteredClaims: jwt.RegisteredClaims{Subject: "user-300"},
		}

		token, err := j.GenerateWithDuration(claims, 30*time.Minute)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		validated, err := j.Validate(t.Context(), token)
		require.NoError(t, err)
		exp, err := validated.GetExpirationTime()
		require.NoError(t, err)
		iat, err := validated.GetIssuedAt()
		require.NoError(t, err)
		require.NotNil(t, exp)
		require.NotNil(t, iat)
		assert.WithinDuration(t, time.Now().Add(30*time.Minute), exp.Time, 3*time.Second)
	})

	t.Run("map claims", func(t *testing.T) {
		claims := jwt.MapClaims{"sub": "user-301"}

		token, err := j.GenerateWithDuration(claims, 15*time.Minute)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		validated, err := j.ValidateWithClaims(t.Context(), token, jwt.MapClaims{})
		require.NoError(t, err)
		exp, err := validated.GetExpirationTime()
		require.NoError(t, err)
		require.NotNil(t, exp)
		assert.WithinDuration(t, time.Now().Add(15*time.Minute), exp.Time, 3*time.Second)
	})

	t.Run("invalid duration", func(t *testing.T) {
		_, err := j.GenerateWithDuration(&StandardClaims{}, 0)
		assert.ErrorIs(t, err, ErrClaimsInvalid)
	})
}

func TestJWT_GenerateWithTokenStoreCompletesIssuedAt(t *testing.T) {
	store, err := cache.NewMemoryCache(cache.NewMemoryConfig(), testx.NopLogger())
	require.NoError(t, err)
	j := MustNew(
		WithSecretKey("test-secret-key-for-testing-32b!"),
		WithLogger(testx.NopLogger()),
		WithTokenStore(CacheTokenStore(store)),
	)
	claims := &StandardClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-302",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	token, err := j.Generate(t.Context(), claims)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	validated, err := j.Validate(t.Context(), token)
	require.NoError(t, err)
	iat, err := validated.GetIssuedAt()
	require.NoError(t, err)
	require.NotNil(t, iat)
}

func TestJWT_GenerateWithDurationContextUsesTokenStore(t *testing.T) {
	store, err := cache.NewMemoryCache(cache.NewMemoryConfig(), testx.NopLogger())
	require.NoError(t, err)
	j := MustNew(
		WithSecretKey("test-secret-key-for-testing-32b!"),
		WithLogger(testx.NopLogger()),
		WithTokenStore(CacheTokenStore(store)),
	)
	claims := &StandardClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: "user-303"},
	}

	token, err := j.GenerateWithDurationContext(t.Context(), claims, time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	validated, err := j.Validate(t.Context(), token)
	require.NoError(t, err)
	subject, err := validated.GetSubject()
	require.NoError(t, err)
	assert.Equal(t, "user-303", subject)
}

func TestJWT_ValidateCachedTokenFailClose(t *testing.T) {
	storeErr := errors.New("store unavailable")
	j := MustNew(
		WithSecretKey("test-secret-key-for-testing-32b!"),
		WithLogger(testx.NopLogger()),
		WithTokenStore(&failingTokenStore{err: storeErr}),
		WithRevokeFailClose(),
	)
	claims := &StandardClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-400",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token, err := j.Generate(t.Context(), claims)
	require.NoError(t, err)

	_, err = j.Validate(t.Context(), token)
	assert.ErrorIs(t, err, ErrTokenStoreQuery)
	assert.ErrorIs(t, err, storeErr)
}

func TestJWT_Accessors(t *testing.T) {
	j := newTestJWT()
	assert.Equal(t, "test-issuer", j.Issuer())
	assert.Equal(t, "JWT", j.Name())
	assert.Equal(t, 2*time.Hour, j.AccessDuration())
	assert.Equal(t, 7*24*time.Hour, j.RefreshDuration())
}

func TestWhitelist(t *testing.T) {
	t.Run("nil whitelist", func(t *testing.T) {
		var w *Whitelist
		assert.False(t, w.IsWhitelisted(t.Context(), nil))
	})

	t.Run("HTTP path matching", func(t *testing.T) {
		w := NewWhitelist().AddHTTPPaths("/health", "/public/")
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		assert.True(t, w.IsWhitelisted(t.Context(), req))

		req2 := httptest.NewRequest(http.MethodGet, "/public/images/logo.png", nil)
		assert.True(t, w.IsWhitelisted(t.Context(), req2))

		req3 := httptest.NewRequest(http.MethodGet, "/api/private", nil)
		assert.False(t, w.IsWhitelisted(t.Context(), req3))
	})

	t.Run("custom internal service header", func(t *testing.T) {
		w := NewWhitelist().SetInternalServiceHeader("x-internal").SetInternalServiceSecret("service-a")
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("x-internal", "service-a")
		assert.True(t, w.IsWhitelisted(t.Context(), req))
	})
}

func TestMustNew_Panics(t *testing.T) {
	t.Run("no secret key", func(t *testing.T) {
		assert.Panics(t, func() {
			MustNew(WithLogger(testx.NopLogger()))
		})
	})

	t.Run("secret key too short", func(t *testing.T) {
		assert.Panics(t, func() {
			MustNew(WithSecretKey("short"), WithLogger(testx.NopLogger()))
		})
	})

	t.Run("no logger uses nop", func(t *testing.T) {
		assert.NotPanics(t, func() {
			MustNew(WithSecretKey("a-key-that-is-at-least-32-bytes!"))
		})
	})
}

func TestNew_ReturnsErrorInsteadOfPanic(t *testing.T) {
	t.Run("no secret key", func(t *testing.T) {
		j, err := New(WithLogger(testx.NopLogger()))
		assert.Nil(t, j)
		assert.Error(t, err)
	})

	t.Run("secret key too short", func(t *testing.T) {
		j, err := New(WithSecretKey("short"), WithLogger(testx.NopLogger()))
		assert.Nil(t, j)
		assert.Error(t, err)
	})

	t.Run("valid config", func(t *testing.T) {
		j, err := New(WithSecretKey("a-key-that-is-at-least-32-bytes!"), WithLogger(testx.NopLogger()))
		assert.NoError(t, err)
		assert.NotNil(t, j)
	})
}

func TestContextFunctions(t *testing.T) {
	t.Run("Claims 上下文操作", func(t *testing.T) {
		claims := &StandardClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject: "user-123",
			},
		}

		ctx := ContextWithClaims(t.Context(), claims)

		retrieved, ok := ClaimsFromContext(ctx)
		assert.True(t, ok)
		assert.NotNil(t, retrieved)

		subject, err := retrieved.GetSubject()
		assert.NoError(t, err)
		assert.Equal(t, "user-123", subject)
	})

	t.Run("Token 上下文操作", func(t *testing.T) {
		ctx := ContextWithToken(t.Context(), "test-token")

		token, ok := TokenFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, "test-token", token)
	})

	t.Run("获取 Subject", func(t *testing.T) {
		claims := &StandardClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject: "user-456",
			},
		}
		ctx := ContextWithClaims(t.Context(), claims)

		subject, ok := GetSubjectFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, "user-456", subject)
	})

	t.Run("无 Claims 获取 Subject", func(t *testing.T) {
		subject, ok := GetSubjectFromContext(t.Context())
		assert.False(t, ok)
		assert.Empty(t, subject)
	})
}
