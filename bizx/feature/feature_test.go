package feature

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnabled_Global(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := t.Context()

	// 全局启用，无限制条件
	err := mgr.SetFlag(ctx, &Flag{Name: "feature-a", Enabled: true})
	require.NoError(t, err)

	assert.True(t, mgr.IsEnabled(ctx, "feature-a"))
}

func TestDisabled(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := t.Context()

	// 全局禁用
	err := mgr.SetFlag(ctx, &Flag{Name: "feature-b", Enabled: false})
	require.NoError(t, err)

	assert.False(t, mgr.IsEnabled(ctx, "feature-b"))

	// 不存在的开关
	assert.False(t, mgr.IsEnabled(ctx, "nonexistent"))
}

func TestEnabled_UserWhitelist(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := t.Context()

	err := mgr.SetFlag(ctx, &Flag{
		Name:    "feature-c",
		Enabled: true,
		Users:   []string{"alice", "bob"},
	})
	require.NoError(t, err)

	// 白名单用户
	assert.True(t, mgr.IsEnabled(ctx, "feature-c", WithUser("alice")))
	assert.True(t, mgr.IsEnabled(ctx, "feature-c", WithUser("bob")))
	// 非白名单用户
	assert.False(t, mgr.IsEnabled(ctx, "feature-c", WithUser("charlie")))
}

func TestEnabled_GroupWhitelist(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := t.Context()

	err := mgr.SetFlag(ctx, &Flag{
		Name:    "feature-d",
		Enabled: true,
		Groups:  []string{"beta-testers"},
	})
	require.NoError(t, err)

	assert.True(t, mgr.IsEnabled(ctx, "feature-d", WithGroup("beta-testers")))
	assert.False(t, mgr.IsEnabled(ctx, "feature-d", WithGroup("normal")))
}

func TestEnabled_Percentage(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := t.Context()

	// 50% 放量
	err := mgr.SetFlag(ctx, &Flag{
		Name:       "feature-e",
		Enabled:    true,
		Percentage: 50,
	})
	require.NoError(t, err)

	// 用大量用户测试分布
	enabled := 0
	total := 1000
	for i := 0; i < total; i++ {
		userID := "user-" + string(rune('A'+i%26)) + string(rune('0'+i%10))
		if mgr.IsEnabled(ctx, "feature-e", WithUser(userID)) {
			enabled++
		}
	}

	// 允许一定误差
	ratio := float64(enabled) / float64(total)
	assert.InDelta(t, 0.5, ratio, 0.15, "百分比放量应该约 50%%，实际 %.2f", ratio)

	// 100% 放量
	err = mgr.SetFlag(ctx, &Flag{
		Name:       "feature-f",
		Enabled:    true,
		Percentage: 100,
	})
	require.NoError(t, err)
	assert.True(t, mgr.IsEnabled(ctx, "feature-f", WithUser("anyone")))
}

func TestSetAndGet(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := t.Context()

	flag := &Flag{
		Name:    "feature-g",
		Enabled: true,
		Metadata: map[string]any{
			"description": "test feature",
		},
	}
	err := mgr.SetFlag(ctx, flag)
	require.NoError(t, err)

	got, err := mgr.GetFlag(ctx, "feature-g")
	require.NoError(t, err)
	assert.Equal(t, "feature-g", got.Name)
	assert.True(t, got.Enabled)

	flags, err := mgr.ListFlags(ctx)
	require.NoError(t, err)
	assert.Len(t, flags, 1)
}

func TestDelete(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	ctx := t.Context()

	err := mgr.SetFlag(ctx, &Flag{Name: "feature-h", Enabled: true})
	require.NoError(t, err)

	err = mgr.DeleteFlag(ctx, "feature-h")
	require.NoError(t, err)

	_, err = mgr.GetFlag(ctx, "feature-h")
	assert.ErrorIs(t, err, ErrFlagNotFound)

	// 删除不存在的
	err = mgr.DeleteFlag(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrFlagNotFound)
}
