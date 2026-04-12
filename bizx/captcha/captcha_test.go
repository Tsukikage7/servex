package captcha

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerate(t *testing.T) {
	store := NewMemoryStore()
	m := NewManager(store, WithLength(6), WithCooldown(0))

	code, err := m.Generate(t.Context(), "13800138000")
	require.NoError(t, err)
	assert.NotNil(t, code)
	assert.Equal(t, "13800138000", code.Key)
	assert.Len(t, code.Code, 6)
	assert.False(t, code.ExpiresAt.IsZero())

	// 验证纯数字
	for _, c := range code.Code {
		assert.True(t, c >= '0' && c <= '9', "应为纯数字")
	}
}

func TestVerify_Valid(t *testing.T) {
	store := NewMemoryStore()
	m := NewManager(store, WithCooldown(0))

	code, err := m.Generate(t.Context(), "test@example.com")
	require.NoError(t, err)

	err = m.Verify(t.Context(), "test@example.com", code.Code)
	assert.NoError(t, err)

	// 验证后验证码应失效
	err = m.Verify(t.Context(), "test@example.com", code.Code)
	assert.ErrorIs(t, err, ErrCodeExpired)
}

func TestVerify_Invalid(t *testing.T) {
	store := NewMemoryStore()
	m := NewManager(store, WithCooldown(0))

	_, err := m.Generate(t.Context(), "test@example.com")
	require.NoError(t, err)

	err = m.Verify(t.Context(), "test@example.com", "000000")
	assert.ErrorIs(t, err, ErrCodeInvalid)
}

func TestVerify_Expired(t *testing.T) {
	store := NewMemoryStore()
	m := NewManager(store, WithExpiration(1*time.Millisecond), WithCooldown(0))

	_, err := m.Generate(t.Context(), "test@example.com")
	require.NoError(t, err)

	// 等待过期
	time.Sleep(10 * time.Millisecond)

	err = m.Verify(t.Context(), "test@example.com", "123456")
	assert.ErrorIs(t, err, ErrCodeExpired)
}

func TestMaxAttempts(t *testing.T) {
	store := NewMemoryStore()
	m := NewManager(store, WithMaxAttempts(3), WithCooldown(0))

	_, err := m.Generate(t.Context(), "test@example.com")
	require.NoError(t, err)

	// 连续错误尝试
	for i := 0; i < 3; i++ {
		err = m.Verify(t.Context(), "test@example.com", "wrong")
		assert.ErrorIs(t, err, ErrCodeInvalid)
	}

	// 超过最大尝试次数
	err = m.Verify(t.Context(), "test@example.com", "wrong")
	assert.ErrorIs(t, err, ErrTooManyAttempts)
}

func TestCooldown(t *testing.T) {
	store := NewMemoryStore()
	m := NewManager(store, WithCooldown(100*time.Millisecond))

	_, err := m.Generate(t.Context(), "test@example.com")
	require.NoError(t, err)

	// 冷却期内再次请求应失败
	_, err = m.Generate(t.Context(), "test@example.com")
	assert.ErrorIs(t, err, ErrCooldown)

	// 等待冷却结束
	time.Sleep(150 * time.Millisecond)

	_, err = m.Generate(t.Context(), "test@example.com")
	assert.NoError(t, err)
}

func TestInvalidate(t *testing.T) {
	store := NewMemoryStore()
	m := NewManager(store, WithCooldown(0))

	code, err := m.Generate(t.Context(), "test@example.com")
	require.NoError(t, err)

	err = m.Invalidate(t.Context(), "test@example.com")
	require.NoError(t, err)

	// 验证应失败
	err = m.Verify(t.Context(), "test@example.com", code.Code)
	assert.ErrorIs(t, err, ErrCodeExpired)
}
