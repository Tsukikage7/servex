package counter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCounter() Counter {
	return NewMemoryCounter(WithPrefix("test:"))
}

func TestIncr(t *testing.T) {
	ctx := t.Context()
	c := newTestCounter()

	val, err := c.Incr(ctx, "hits", 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), val)

	val, err = c.Incr(ctx, "hits", 5)
	require.NoError(t, err)
	assert.Equal(t, int64(6), val)

	// 负数增量
	val, err = c.Incr(ctx, "hits", -2)
	require.NoError(t, err)
	assert.Equal(t, int64(4), val)
}

func TestGet(t *testing.T) {
	ctx := t.Context()
	c := newTestCounter()

	// 不存在的键返回 0
	val, err := c.Get(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Equal(t, int64(0), val)

	_, _ = c.Incr(ctx, "counter", 10)
	val, err = c.Get(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(10), val)
}

func TestReset(t *testing.T) {
	ctx := t.Context()
	c := newTestCounter()

	_, _ = c.Incr(ctx, "counter", 10)
	err := c.Reset(ctx, "counter")
	require.NoError(t, err)

	val, err := c.Get(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(0), val)
}

func TestIncrWindow(t *testing.T) {
	ctx := t.Context()
	c := newTestCounter()

	window := 1 * time.Second

	count, err := c.IncrWindow(ctx, "api", window)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	count, err = c.IncrWindow(ctx, "api", window)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	count, err = c.IncrWindow(ctx, "api", window)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestGetWindow(t *testing.T) {
	ctx := t.Context()
	c := newTestCounter()

	window := 500 * time.Millisecond

	_, _ = c.IncrWindow(ctx, "req", window)
	_, _ = c.IncrWindow(ctx, "req", window)

	count, err := c.GetWindow(ctx, "req", window)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// 等待窗口过期
	time.Sleep(600 * time.Millisecond)

	count, err = c.GetWindow(ctx, "req", window)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestMGet(t *testing.T) {
	ctx := t.Context()
	c := newTestCounter()

	_, _ = c.Incr(ctx, "a", 1)
	_, _ = c.Incr(ctx, "b", 2)
	_, _ = c.Incr(ctx, "c", 3)

	result, err := c.MGet(ctx, "a", "b", "c", "d")
	require.NoError(t, err)
	assert.Equal(t, int64(1), result["a"])
	assert.Equal(t, int64(2), result["b"])
	assert.Equal(t, int64(3), result["c"])
	assert.Equal(t, int64(0), result["d"]) // 不存在的键返回 0
}

func TestConcurrentIncr(t *testing.T) {
	ctx := t.Context()
	c := newTestCounter()

	const goroutines = 50
	const incrsPerGoroutine = 100

	done := make(chan struct{})
	for range goroutines {
		go func() {
			for range incrsPerGoroutine {
				_, _ = c.Incr(ctx, "concurrent", 1)
			}
			done <- struct{}{}
		}()
	}

	for range goroutines {
		<-done
	}

	val, err := c.Get(ctx, "concurrent")
	require.NoError(t, err)
	assert.Equal(t, int64(goroutines*incrsPerGoroutine), val)
}

func TestWindowExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping window expiry test in short mode")
	}

	ctx := t.Context()
	c := newTestCounter()

	window := 200 * time.Millisecond

	_, _ = c.IncrWindow(ctx, "expiry", window)
	_, _ = c.IncrWindow(ctx, "expiry", window)

	count, err := c.GetWindow(ctx, "expiry", window)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	time.Sleep(300 * time.Millisecond)

	count, err = c.GetWindow(ctx, "expiry", window)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// New increment after expiry.
	count, err = c.IncrWindow(ctx, "expiry", window)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestNoPrefix(t *testing.T) {
	ctx := t.Context()
	c := NewMemoryCounter() // no prefix

	val, err := c.Incr(ctx, "key", 5)
	require.NoError(t, err)
	assert.Equal(t, int64(5), val)

	got, err := c.Get(ctx, "key")
	require.NoError(t, err)
	assert.Equal(t, int64(5), got)
}

func TestResetAlsoClearsWindow(t *testing.T) {
	ctx := t.Context()
	c := newTestCounter()

	_, _ = c.IncrWindow(ctx, "wr", 10*time.Second)
	_, _ = c.IncrWindow(ctx, "wr", 10*time.Second)

	err := c.Reset(ctx, "wr")
	require.NoError(t, err)

	count, err := c.GetWindow(ctx, "wr", 10*time.Second)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestMGetEmpty(t *testing.T) {
	ctx := t.Context()
	c := newTestCounter()

	result, err := c.MGet(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, len(result))
}

func TestGetWindowNonexistent(t *testing.T) {
	ctx := t.Context()
	c := newTestCounter()

	count, err := c.GetWindow(ctx, "nokey", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}
