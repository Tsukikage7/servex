package sequence

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNext_WithDate(t *testing.T) {
	store := NewMemoryStore()
	seq := New(&Config{
		Name:       "order",
		Prefix:     "ORD-",
		DateFormat: "20060102",
		Padding:    4,
	}, store)

	ctx := t.Context()
	id, err := seq.Next(ctx)
	require.NoError(t, err)

	today := time.Now().Format("20060102")
	assert.True(t, strings.HasPrefix(id, "ORD-"+today+"-"))
	assert.True(t, strings.HasSuffix(id, "0001"))

	id2, err := seq.Next(ctx)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(id2, "0002"))
}

func TestNext_WithoutDate(t *testing.T) {
	store := NewMemoryStore()
	seq := New(&Config{
		Name:    "invoice",
		Prefix:  "INV-",
		Padding: 6,
	}, store)

	ctx := t.Context()
	id, err := seq.Next(ctx)
	require.NoError(t, err)
	assert.Equal(t, "INV-000001", id)

	id2, err := seq.Next(ctx)
	require.NoError(t, err)
	assert.Equal(t, "INV-000002", id2)
}

func TestNext_ResetDaily(t *testing.T) {
	store := NewMemoryStore()
	seq := New(&Config{
		Name:       "daily",
		Prefix:     "D-",
		DateFormat: "20060102",
		Padding:    3,
		ResetDaily: true,
	}, store)

	ctx := t.Context()
	id, err := seq.Next(ctx)
	require.NoError(t, err)

	today := time.Now().Format("20060102")
	assert.True(t, strings.HasPrefix(id, "D-"+today+"-"))
	assert.True(t, strings.HasSuffix(id, "001"))
}

func TestCurrent(t *testing.T) {
	store := NewMemoryStore()
	seq := New(&Config{
		Name:    "test",
		Prefix:  "T-",
		Padding: 3,
	}, store)

	ctx := t.Context()

	// 初始状态
	cur, err := seq.Current(ctx)
	require.NoError(t, err)
	assert.Equal(t, "T-000", cur)

	// 生成一个后
	_, _ = seq.Next(ctx)
	cur, err = seq.Current(ctx)
	require.NoError(t, err)
	assert.Equal(t, "T-001", cur)
}

func TestReset(t *testing.T) {
	store := NewMemoryStore()
	seq := New(&Config{
		Name:    "reset",
		Prefix:  "R-",
		Padding: 3,
	}, store)

	ctx := t.Context()
	_, _ = seq.Next(ctx)
	_, _ = seq.Next(ctx)

	err := seq.Reset(ctx)
	require.NoError(t, err)

	id, err := seq.Next(ctx)
	require.NoError(t, err)
	assert.Equal(t, "R-001", id)
}

func TestPadding(t *testing.T) {
	store := NewMemoryStore()
	seq := New(&Config{
		Name:    "pad",
		Prefix:  "",
		Padding: 8,
	}, store)

	ctx := t.Context()
	id, err := seq.Next(ctx)
	require.NoError(t, err)
	assert.Equal(t, "00000001", id)
	assert.Len(t, id, 8)
}
