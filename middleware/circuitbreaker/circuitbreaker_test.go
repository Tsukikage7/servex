package circuitbreaker

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errTest = errors.New("test error")

func TestBreaker_InitialState(t *testing.T) {
	b := New()
	assert.Equal(t, StateClosed, b.State())
}

func TestBreaker_Execute_Success(t *testing.T) {
	b := New()
	err := b.Execute(t.Context(), func() error { return nil })
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, b.State())
}

func TestBreaker_Execute_FailuresToOpen(t *testing.T) {
	b := New(WithFailureThreshold(3))

	for range 3 {
		err := b.Execute(t.Context(), func() error { return errTest })
		assert.Error(t, err)
	}

	assert.Equal(t, StateOpen, b.State())
}

func TestBreaker_Execute_OpenRejectsRequests(t *testing.T) {
	b := New(WithFailureThreshold(1))

	// 触发开路
	b.Execute(t.Context(), func() error { return errTest })
	assert.Equal(t, StateOpen, b.State())

	// 开路时请求应被拒绝
	err := b.Execute(t.Context(), func() error { return nil })
	assert.ErrorIs(t, err, ErrCircuitOpen)
}

func TestBreaker_OpenToHalfOpen(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := New(
			WithFailureThreshold(1),
			WithOpenTimeout(50*time.Millisecond),
		)

		b.Execute(t.Context(), func() error { return errTest })
		assert.Equal(t, StateOpen, b.State())

		// synctest 虚拟时钟：立即推进，无需真实等待
		time.Sleep(60 * time.Millisecond)

		assert.Equal(t, StateHalfOpen, b.State())
	})
}

func TestBreaker_HalfOpenToClosedOnSuccess(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := New(
			WithFailureThreshold(1),
			WithSuccessThreshold(2),
			WithOpenTimeout(50*time.Millisecond),
		)

		// 触发开路
		b.Execute(t.Context(), func() error { return errTest })

		// synctest 虚拟时钟：立即推进到 HalfOpen
		time.Sleep(60 * time.Millisecond)
		require.Equal(t, StateHalfOpen, b.State())

		// 成功两次后关路
		b.Execute(t.Context(), func() error { return nil })
		b.Execute(t.Context(), func() error { return nil })

		assert.Equal(t, StateClosed, b.State())
	})
}

func TestBreaker_HalfOpenToOpenOnFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := New(
			WithFailureThreshold(1),
			WithOpenTimeout(50*time.Millisecond),
		)

		b.Execute(t.Context(), func() error { return errTest })

		// synctest 虚拟时钟：立即推进到 HalfOpen
		time.Sleep(60 * time.Millisecond)
		require.Equal(t, StateHalfOpen, b.State())

		// 探测失败，重新开路
		b.Execute(t.Context(), func() error { return errTest })

		assert.Equal(t, StateOpen, b.State())
	})
}

func TestBreaker_Reset(t *testing.T) {
	b := New(WithFailureThreshold(1))

	b.Execute(t.Context(), func() error { return errTest })
	assert.Equal(t, StateOpen, b.State())

	b.Reset()
	assert.Equal(t, StateClosed, b.State())
}

func TestBreaker_FailureCountResetOnSuccess(t *testing.T) {
	b := New(WithFailureThreshold(3))

	b.Execute(t.Context(), func() error { return errTest })
	b.Execute(t.Context(), func() error { return errTest })
	// 成功后重置计数
	b.Execute(t.Context(), func() error { return nil })
	// 再次失败不应立即开路（计数从 0 重新开始）
	b.Execute(t.Context(), func() error { return errTest })
	b.Execute(t.Context(), func() error { return errTest })

	assert.Equal(t, StateClosed, b.State())
}

func TestBreaker_CustomIsFailure(t *testing.T) {
	b := New(
		WithFailureThreshold(1),
		// 只有特定错误才算失败
		WithIsFailure(func(err error) bool {
			return errors.Is(err, errTest)
		}),
	)

	// 普通错误不触发熔断
	b.Execute(t.Context(), func() error { return errors.New("other error") })
	assert.Equal(t, StateClosed, b.State())

	// 特定错误触发熔断
	b.Execute(t.Context(), func() error { return errTest })
	assert.Equal(t, StateOpen, b.State())
}
