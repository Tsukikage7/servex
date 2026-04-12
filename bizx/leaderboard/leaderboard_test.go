package leaderboard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLB() Leaderboard {
	return NewMemoryLeaderboard("test", WithPrefix("test:"))
}

func TestAddAndTopN(t *testing.T) {
	ctx := t.Context()
	lb := newTestLB()

	require.NoError(t, lb.AddScore(ctx, "alice", 100))
	require.NoError(t, lb.AddScore(ctx, "bob", 200))
	require.NoError(t, lb.AddScore(ctx, "charlie", 150))

	top, err := lb.TopN(ctx, 2)
	require.NoError(t, err)
	require.Len(t, top, 2)
	assert.Equal(t, "bob", top[0].Member)
	assert.Equal(t, float64(200), top[0].Score)
	assert.Equal(t, int64(1), top[0].Rank)
	assert.Equal(t, "charlie", top[1].Member)
	assert.Equal(t, int64(2), top[1].Rank)
}

func TestIncrScore(t *testing.T) {
	ctx := t.Context()
	lb := newTestLB()

	require.NoError(t, lb.AddScore(ctx, "alice", 100))
	newScore, err := lb.IncrScore(ctx, "alice", 50)
	require.NoError(t, err)
	assert.Equal(t, float64(150), newScore)

	score, err := lb.GetScore(ctx, "alice")
	require.NoError(t, err)
	assert.Equal(t, float64(150), score)
}

func TestGetRank(t *testing.T) {
	ctx := t.Context()
	lb := newTestLB()

	require.NoError(t, lb.AddScore(ctx, "alice", 100))
	require.NoError(t, lb.AddScore(ctx, "bob", 200))
	require.NoError(t, lb.AddScore(ctx, "charlie", 150))

	entry, err := lb.GetRank(ctx, "charlie")
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, int64(2), entry.Rank)
	assert.Equal(t, float64(150), entry.Score)

	// 不存在的成员
	entry, err = lb.GetRank(ctx, "nobody")
	require.NoError(t, err)
	assert.Nil(t, entry)
}

func TestGetPage(t *testing.T) {
	ctx := t.Context()
	lb := newTestLB()

	for i := 0; i < 10; i++ {
		require.NoError(t, lb.AddScore(ctx, "p"+string(rune('a'+i)), float64(i*10)))
	}

	page, err := lb.GetPage(ctx, 0, 3)
	require.NoError(t, err)
	assert.Equal(t, int64(10), page.Total)
	assert.Len(t, page.Entries, 3)
	assert.True(t, page.HasMore)

	// 最后一页
	page, err = lb.GetPage(ctx, 9, 3)
	require.NoError(t, err)
	assert.Len(t, page.Entries, 1)
	assert.False(t, page.HasMore)

	// 超出范围
	page, err = lb.GetPage(ctx, 20, 3)
	require.NoError(t, err)
	assert.Len(t, page.Entries, 0)
}

func TestReset(t *testing.T) {
	ctx := t.Context()
	lb := newTestLB()

	require.NoError(t, lb.AddScore(ctx, "alice", 100))
	require.NoError(t, lb.Reset(ctx))

	count, err := lb.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestAscending(t *testing.T) {
	ctx := t.Context()
	lb := NewMemoryLeaderboard("asc", WithOrder(Ascending))

	require.NoError(t, lb.AddScore(ctx, "alice", 100))
	require.NoError(t, lb.AddScore(ctx, "bob", 50))
	require.NoError(t, lb.AddScore(ctx, "charlie", 150))

	top, err := lb.TopN(ctx, 3)
	require.NoError(t, err)
	assert.Equal(t, "bob", top[0].Member) // 最低分排第一
	assert.Equal(t, "alice", top[1].Member)
	assert.Equal(t, "charlie", top[2].Member)
}
