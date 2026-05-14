package errors

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	err := New(100401, "auth.token.expired", "令牌已过期")
	assert.Equal(t, 100401, err.Code)
	assert.Equal(t, "auth.token.expired", err.Key)
	assert.Equal(t, "令牌已过期", err.Message)
	assert.Equal(t, KindInternal, err.Kind)
	assert.Nil(t, err.Metadata)
	assert.Nil(t, err.cause)
}

func TestNewWithKind(t *testing.T) {
	err := NewWithKind(200404, "user.not_found", "用户不存在", KindNotFound)

	assert.Equal(t, 200404, err.Code)
	assert.Equal(t, "user.not_found", err.Key)
	assert.Equal(t, "用户不存在", err.Message)
	assert.Equal(t, KindNotFound, err.Kind)
}

func TestKindMappings(t *testing.T) {
	cases := []struct {
		name string
		kind Kind
		http int
	}{
		{"internal", KindInternal, http.StatusInternalServerError},
		{"unknown", KindUnknown, http.StatusInternalServerError},
		{"canceled", KindCanceled, http.StatusRequestTimeout},
		{"not found", KindNotFound, http.StatusNotFound},
		{"conflict", KindConflict, http.StatusConflict},
		{"invalid argument", KindInvalidArgument, http.StatusBadRequest},
		{"permission denied", KindPermissionDenied, http.StatusForbidden},
		{"unauthenticated", KindUnauthenticated, http.StatusUnauthorized},
		{"failed precondition", KindFailedPrecondition, http.StatusPreconditionFailed},
		{"unavailable", KindUnavailable, http.StatusServiceUnavailable},
		{"deadline exceeded", KindDeadlineExceeded, http.StatusGatewayTimeout},
		{"resource exhausted", KindResourceExhausted, http.StatusTooManyRequests},
		{"not implemented", KindNotImplemented, http.StatusNotImplemented},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.http, tt.kind.HTTPStatus())
		})
	}
}

func TestError_Error(t *testing.T) {
	err := New(100401, "auth.token.expired", "令牌已过期")
	assert.Equal(t, "[100401] auth.token.expired: 令牌已过期", err.Error())
}

func TestError_WithKind(t *testing.T) {
	original := New(300001, "project.slug_required", "Slug 不能为空")
	err := original.WithKind(KindInvalidArgument)

	assert.Equal(t, KindInvalidArgument, err.Kind)
	assert.Equal(t, KindInternal, original.Kind)
}

func TestError_ChainedBuilder(t *testing.T) {
	err := New(100401, "auth.token.expired", "令牌已过期").
		WithKind(KindUnauthenticated)
	assert.Equal(t, 100401, err.Code)
	assert.Equal(t, KindUnauthenticated, err.Kind)
}

func TestError_WithCause(t *testing.T) {
	original := NewWithKind(100401, "auth.token.expired", "令牌已过期", KindUnauthenticated)
	cause := fmt.Errorf("token parse failed")
	wrapped := original.WithCause(cause)

	assert.Equal(t, "[100401] auth.token.expired: 令牌已过期: token parse failed", wrapped.Error())
	assert.ErrorIs(t, wrapped, cause)

	assert.Equal(t, "[100401] auth.token.expired: 令牌已过期", original.Error())
	assert.Nil(t, original.cause)
}

func TestError_WithMeta(t *testing.T) {
	original := New(100401, "auth.token.expired", "令牌已过期")
	withMeta := original.WithMeta("user_id", "123")

	assert.Equal(t, "123", withMeta.Metadata["user_id"])
	assert.Nil(t, original.Metadata)
}

func TestError_WithMeta_Multiple(t *testing.T) {
	err := New(100401, "auth.token.expired", "令牌已过期").
		WithMeta("user_id", "123").
		WithMeta("ip", "1.2.3.4")

	assert.Equal(t, "123", err.Metadata["user_id"])
	assert.Equal(t, "1.2.3.4", err.Metadata["ip"])
}

func TestError_WithMessage(t *testing.T) {
	original := NewWithKind(100401, "auth.token.expired", "令牌已过期", KindUnauthenticated)
	replaced := original.WithMessage("Token expired")

	assert.Equal(t, "Token expired", replaced.Message)
	assert.Equal(t, "令牌已过期", original.Message)
	assert.Equal(t, KindUnauthenticated, replaced.Kind)
}

func TestError_Is_StandardLibrary(t *testing.T) {
	ErrAuth := New(100401, "auth.token.expired", "令牌已过期")
	wrapped := ErrAuth.WithCause(fmt.Errorf("bad token"))

	assert.True(t, errors.Is(wrapped, ErrAuth))

	ErrOther := New(200404, "user.not_found", "用户不存在")
	assert.False(t, errors.Is(wrapped, ErrOther))
}

func TestError_As(t *testing.T) {
	ErrAuth := NewWithKind(100401, "auth.token.expired", "令牌已过期", KindUnauthenticated)
	wrapped := fmt.Errorf("outer: %w", ErrAuth)

	var target *Error
	require.True(t, errors.As(wrapped, &target))
	assert.Equal(t, 100401, target.Code)
	assert.Equal(t, KindUnauthenticated, target.Kind)
}

func TestFromError(t *testing.T) {
	t.Run("from *Error", func(t *testing.T) {
		err := New(100401, "auth.token.expired", "令牌已过期")
		got, ok := FromError(err)
		assert.True(t, ok)
		assert.Equal(t, 100401, got.Code)
	})

	t.Run("from wrapped *Error", func(t *testing.T) {
		err := New(100401, "auth.token.expired", "令牌已过期")
		wrapped := fmt.Errorf("outer: %w", err)
		got, ok := FromError(wrapped)
		assert.True(t, ok)
		assert.Equal(t, 100401, got.Code)
	})

	t.Run("from WithCause", func(t *testing.T) {
		err := New(100401, "auth.token.expired", "令牌已过期").WithCause(fmt.Errorf("bad"))
		got, ok := FromError(err)
		assert.True(t, ok)
		assert.Equal(t, 100401, got.Code)
	})

	t.Run("from nil", func(t *testing.T) {
		_, ok := FromError(nil)
		assert.False(t, ok)
	})

	t.Run("from standard error", func(t *testing.T) {
		_, ok := FromError(fmt.Errorf("plain error"))
		assert.False(t, ok)
	})
}

func TestCodeIs(t *testing.T) {
	ErrAuth := New(100401, "auth.token.expired", "令牌已过期")

	t.Run("direct match", func(t *testing.T) {
		assert.True(t, CodeIs(ErrAuth, ErrAuth))
	})

	t.Run("wrapped match", func(t *testing.T) {
		wrapped := ErrAuth.WithCause(fmt.Errorf("bad"))
		assert.True(t, CodeIs(wrapped, ErrAuth))
	})

	t.Run("fmt wrapped match", func(t *testing.T) {
		wrapped := fmt.Errorf("outer: %w", ErrAuth)
		assert.True(t, CodeIs(wrapped, ErrAuth))
	})

	t.Run("no match", func(t *testing.T) {
		other := New(200404, "user.not_found", "用户不存在")
		assert.False(t, CodeIs(other, ErrAuth))
	})

	t.Run("nil error", func(t *testing.T) {
		assert.False(t, CodeIs(nil, ErrAuth))
	})
}
