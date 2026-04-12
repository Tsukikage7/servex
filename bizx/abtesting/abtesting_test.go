package abtesting

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestExperiment() *Experiment {
	return &Experiment{
		ID:      "exp-1",
		Name:    "按钮颜色测试",
		Enabled: true,
		Variants: []Variant{
			{ID: "control", Name: "蓝色按钮", Weight: 50},
			{ID: "treatment", Name: "绿色按钮", Weight: 50},
		},
		Salt:      "test-salt",
		StartTime: time.Now().Add(-time.Hour),
		EndTime:   time.Now().Add(24 * time.Hour),
	}
}

func TestAssignmentConsistency(t *testing.T) {
	store := NewMemoryStore()
	mgr := New(store)
	ctx := t.Context()

	err := mgr.CreateExperiment(ctx, newTestExperiment())
	require.NoError(t, err)

	// 同一用户多次分配应得到相同结果
	a1, err := mgr.Assign(ctx, "exp-1", "user-1")
	require.NoError(t, err)

	a2, err := mgr.Assign(ctx, "exp-1", "user-1")
	require.NoError(t, err)

	assert.Equal(t, a1.VariantID, a2.VariantID)
	assert.Equal(t, "exp-1", a1.ExperimentID)
	assert.Equal(t, "user-1", a1.UserID)
}

func TestWeightDistribution(t *testing.T) {
	store := NewMemoryStore()
	mgr := New(store)
	ctx := t.Context()

	exp := &Experiment{
		ID:      "exp-dist",
		Name:    "分布测试",
		Enabled: true,
		Variants: []Variant{
			{ID: "a", Name: "变体A", Weight: 70},
			{ID: "b", Name: "变体B", Weight: 30},
		},
		Salt:      "dist-salt",
		StartTime: time.Now().Add(-time.Hour),
		EndTime:   time.Now().Add(24 * time.Hour),
	}
	err := mgr.CreateExperiment(ctx, exp)
	require.NoError(t, err)

	counts := map[string]int{"a": 0, "b": 0}
	total := 10000
	for i := 0; i < total; i++ {
		userID := fmt.Sprintf("user-%d", i)
		a, err := mgr.Assign(ctx, "exp-dist", userID)
		require.NoError(t, err)
		counts[a.VariantID]++
	}

	ratioA := float64(counts["a"]) / float64(total)
	assert.InDelta(t, 0.70, ratioA, 0.05, "变体A应约70%%，实际 %.2f", ratioA)
}

func TestEnableDisable(t *testing.T) {
	store := NewMemoryStore()
	mgr := New(store)
	ctx := t.Context()

	err := mgr.CreateExperiment(ctx, newTestExperiment())
	require.NoError(t, err)

	// 分配成功
	_, err = mgr.Assign(ctx, "exp-1", "user-1")
	require.NoError(t, err)

	// 禁用实验
	err = mgr.DisableExperiment(ctx, "exp-1")
	require.NoError(t, err)

	exp, err := mgr.GetExperiment(ctx, "exp-1")
	require.NoError(t, err)
	assert.False(t, exp.Enabled)

	// 新用户分配应失败
	_, err = mgr.Assign(ctx, "exp-1", "user-new")
	assert.ErrorIs(t, err, ErrExperimentDisabled)
}

func TestExposureTracking(t *testing.T) {
	store := NewMemoryStore()
	mgr := New(store)
	ctx := t.Context()

	err := mgr.CreateExperiment(ctx, newTestExperiment())
	require.NoError(t, err)

	a, err := mgr.Assign(ctx, "exp-1", "user-1")
	require.NoError(t, err)

	event := &ExposureEvent{
		ExperimentID: a.ExperimentID,
		VariantID:    a.VariantID,
		UserID:       a.UserID,
		Timestamp:    time.Now(),
	}
	err = mgr.TrackExposure(ctx, event)
	require.NoError(t, err)
}

func TestCreateExperiment_NoVariants(t *testing.T) {
	store := NewMemoryStore()
	mgr := New(store)
	ctx := t.Context()

	exp := &Experiment{
		ID:      "exp-empty",
		Name:    "空变体实验",
		Enabled: true,
	}
	err := mgr.CreateExperiment(ctx, exp)
	assert.ErrorIs(t, err, ErrNoVariants)
}

func TestCreateExperiment_InvalidWeight(t *testing.T) {
	store := NewMemoryStore()
	mgr := New(store)
	ctx := t.Context()

	// 权重总和不等于 100
	exp := &Experiment{
		ID:      "exp-bad-weight",
		Name:    "权重错误",
		Enabled: true,
		Variants: []Variant{
			{ID: "a", Name: "A", Weight: 30},
			{ID: "b", Name: "B", Weight: 30},
		},
		Salt: "salt",
	}
	err := mgr.CreateExperiment(ctx, exp)
	assert.ErrorIs(t, err, ErrInvalidWeight)

	// 负数权重
	exp2 := &Experiment{
		ID:      "exp-neg-weight",
		Name:    "负权重",
		Enabled: true,
		Variants: []Variant{
			{ID: "a", Name: "A", Weight: -10},
			{ID: "b", Name: "B", Weight: 110},
		},
		Salt: "salt",
	}
	err = mgr.CreateExperiment(ctx, exp2)
	assert.ErrorIs(t, err, ErrInvalidWeight)
}

func TestExperimentNotFound(t *testing.T) {
	store := NewMemoryStore()
	mgr := New(store)
	ctx := t.Context()

	_, err := mgr.GetExperiment(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrExperimentNotFound)

	_, err = mgr.Assign(ctx, "nonexistent", "user-1")
	assert.ErrorIs(t, err, ErrExperimentNotFound)
}

func TestListExperiments(t *testing.T) {
	store := NewMemoryStore()
	mgr := New(store)
	ctx := t.Context()

	err := mgr.CreateExperiment(ctx, newTestExperiment())
	require.NoError(t, err)

	exp2 := newTestExperiment()
	exp2.ID = "exp-2"
	exp2.Name = "第二个实验"
	err = mgr.CreateExperiment(ctx, exp2)
	require.NoError(t, err)

	list, err := mgr.ListExperiments(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestGetAssignment(t *testing.T) {
	store := NewMemoryStore()
	mgr := New(store)
	ctx := t.Context()

	err := mgr.CreateExperiment(ctx, newTestExperiment())
	require.NoError(t, err)

	assigned, err := mgr.Assign(ctx, "exp-1", "user-1")
	require.NoError(t, err)

	got, err := mgr.GetAssignment(ctx, "exp-1", "user-1")
	require.NoError(t, err)
	assert.Equal(t, assigned.VariantID, got.VariantID)

	// 不存在的分配
	_, err = mgr.GetAssignment(ctx, "exp-1", "nonexistent")
	assert.Error(t, err)
}
