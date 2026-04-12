package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newQuota(key string, limit int64) Quota {
	return Quota{
		Key:    key,
		Limit:  limit,
		Window: 1 * time.Hour,
	}
}

func TestConsume(t *testing.T) {
	ctx := t.Context()
	mgr := NewMemoryQuotaManager()

	quota := newQuota("user:1", 10)
	usage, err := mgr.Consume(ctx, quota, 3)
	require.NoError(t, err)
	assert.Equal(t, int64(3), usage.Used)
	assert.Equal(t, int64(7), usage.Remaining)
	assert.Equal(t, int64(10), usage.Limit)

	usage, err = mgr.Consume(ctx, quota, 5)
	require.NoError(t, err)
	assert.Equal(t, int64(8), usage.Used)
	assert.Equal(t, int64(2), usage.Remaining)
}

func TestCheck(t *testing.T) {
	ctx := t.Context()
	mgr := NewMemoryQuotaManager()

	quota := newQuota("user:2", 100)

	// 未使用时检查
	usage, err := mgr.Check(ctx, quota)
	require.NoError(t, err)
	assert.Equal(t, int64(0), usage.Used)
	assert.Equal(t, int64(100), usage.Remaining)

	// 消耗后检查
	_, _ = mgr.Consume(ctx, quota, 30)
	usage, err = mgr.Check(ctx, quota)
	require.NoError(t, err)
	assert.Equal(t, int64(30), usage.Used)
	assert.Equal(t, int64(70), usage.Remaining)
}

func TestQuotaExceeded(t *testing.T) {
	ctx := t.Context()
	mgr := NewMemoryQuotaManager()

	quota := newQuota("user:3", 5)

	_, err := mgr.Consume(ctx, quota, 3)
	require.NoError(t, err)

	// 超出配额
	usage, err := mgr.Consume(ctx, quota, 5)
	assert.ErrorIs(t, err, ErrQuotaExceeded)
	assert.Equal(t, int64(3), usage.Used)
	assert.Equal(t, int64(2), usage.Remaining)

	// 恰好用完
	usage, err = mgr.Consume(ctx, quota, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(5), usage.Used)
	assert.Equal(t, int64(0), usage.Remaining)

	// 再消耗应该失败
	_, err = mgr.Consume(ctx, quota, 1)
	assert.ErrorIs(t, err, ErrQuotaExceeded)
}

func TestResetQuota(t *testing.T) {
	ctx := t.Context()
	mgr := NewMemoryQuotaManager()

	quota := newQuota("user:4", 10)
	_, _ = mgr.Consume(ctx, quota, 8)

	err := mgr.Reset(ctx, "user:4")
	require.NoError(t, err)

	usage, err := mgr.Check(ctx, quota)
	require.NoError(t, err)
	assert.Equal(t, int64(0), usage.Used)
	assert.Equal(t, int64(10), usage.Remaining)
}

func TestGetUsage(t *testing.T) {
	ctx := t.Context()
	mgr := NewMemoryQuotaManager()

	quota := newQuota("user:5", 100)
	_, _ = mgr.Consume(ctx, quota, 42)

	usage, err := mgr.GetUsage(ctx, quota)
	require.NoError(t, err)
	assert.Equal(t, int64(42), usage.Used)
	assert.Equal(t, int64(58), usage.Remaining)
	assert.False(t, usage.ResetsAt.IsZero())
}

func TestWindowReset(t *testing.T) {
	ctx := t.Context()
	mgr := NewMemoryQuotaManager()

	// 使用很短的窗口
	quota := Quota{
		Key:    "user:6",
		Limit:  10,
		Window: 500 * time.Millisecond,
	}

	_, err := mgr.Consume(ctx, quota, 8)
	require.NoError(t, err)

	// 等待窗口过期
	time.Sleep(600 * time.Millisecond)

	// 新窗口应该重置
	usage, err := mgr.Check(ctx, quota)
	require.NoError(t, err)
	assert.Equal(t, int64(0), usage.Used)
	assert.Equal(t, int64(10), usage.Remaining)
}

func TestZeroLimit(t *testing.T) {
	ctx := t.Context()
	mgr := NewMemoryQuotaManager()

	quota := Quota{
		Key:    "user:zero",
		Limit:  0,
		Window: 1 * time.Hour,
	}

	// With zero limit, any consumption should fail
	_, err := mgr.Consume(ctx, quota, 1)
	assert.ErrorIs(t, err, ErrQuotaExceeded)

	// Check should show 0 remaining
	usage, err := mgr.Check(ctx, quota)
	require.NoError(t, err)
	assert.Equal(t, int64(0), usage.Remaining)
}

func TestLargeWindow(t *testing.T) {
	ctx := t.Context()
	mgr := NewMemoryQuotaManager()

	quota := Quota{
		Key:    "user:large",
		Limit:  1000000,
		Window: 720 * time.Hour, // 30 days
	}

	usage, err := mgr.Consume(ctx, quota, 500000)
	require.NoError(t, err)
	assert.Equal(t, int64(500000), usage.Used)
	assert.Equal(t, int64(500000), usage.Remaining)
	assert.False(t, usage.ResetsAt.IsZero())
}

func TestConcurrentAccess(t *testing.T) {
	ctx := t.Context()
	mgr := NewMemoryQuotaManager()

	quota := Quota{
		Key:    "user:concurrent",
		Limit:  1000,
		Window: 1 * time.Hour,
	}

	done := make(chan struct{}, 100)
	for i := 0; i < 100; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_, _ = mgr.Consume(ctx, quota, 1)
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	usage, err := mgr.Check(ctx, quota)
	require.NoError(t, err)
	// All 100 goroutines consumed 1 each
	assert.Equal(t, int64(100), usage.Used)
	assert.Equal(t, int64(900), usage.Remaining)
}

func TestKeyPrefix(t *testing.T) {
	o := applyOptions([]Option{WithKeyPrefix("prefix:")})
	assert.Equal(t, "prefix:", o.keyPrefix)
}
