package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinearWorkflow(t *testing.T) {
	store := NewMemoryStore()
	engine := New(store)
	ctx := context.Background()

	def := &Definition{
		ID:          "linear-1",
		Name:        "线性流程",
		Version:     "1.0",
		StartNodeID: "start",
		Nodes: map[string]*Node{
			"start": {
				ID:   "start",
				Name: "开始任务",
				Type: NodeTypeTask,
				Handler: func(_ context.Context, inst *Instance) error {
					inst.Data["step1"] = true
					return nil
				},
				NextNodes: []string{"step2"},
			},
			"step2": {
				ID:   "step2",
				Name: "第二步",
				Type: NodeTypeTask,
				Handler: func(_ context.Context, inst *Instance) error {
					inst.Data["step2"] = true
					return nil
				},
				NextNodes: []string{"end"},
			},
			"end": {
				ID:   "end",
				Name: "结束",
				Type: NodeTypeEnd,
			},
		},
	}

	err := engine.RegisterDefinition(def)
	require.NoError(t, err)

	inst, err := engine.StartWorkflow(ctx, "linear-1", map[string]any{"init": true})
	require.NoError(t, err)
	assert.Equal(t, StatusPending, inst.Status)

	err = engine.Execute(ctx, inst.ID)
	require.NoError(t, err)

	result, err := engine.GetInstance(ctx, inst.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, result.Status)
	assert.Equal(t, true, result.Data["step1"])
	assert.Equal(t, true, result.Data["step2"])
}

func TestApprovalFlow(t *testing.T) {
	store := NewMemoryStore()
	engine := New(store)
	ctx := context.Background()

	def := &Definition{
		ID:          "approval-1",
		Name:        "审批流程",
		Version:     "1.0",
		StartNodeID: "submit",
		Nodes: map[string]*Node{
			"submit": {
				ID:        "submit",
				Name:      "提交申请",
				Type:      NodeTypeTask,
				NextNodes: []string{"approve"},
			},
			"approve": {
				ID:        "approve",
				Name:      "审批",
				Type:      NodeTypeApproval,
				NextNodes: []string{"end"},
			},
			"end": {
				ID:   "end",
				Name: "结束",
				Type: NodeTypeEnd,
			},
		},
	}

	err := engine.RegisterDefinition(def)
	require.NoError(t, err)

	inst, err := engine.StartWorkflow(ctx, "approval-1", nil)
	require.NoError(t, err)

	// 执行到审批节点
	err = engine.Execute(ctx, inst.ID)
	require.NoError(t, err)

	result, err := engine.GetInstance(ctx, inst.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusWaitingApproval, result.Status)

	// 审批通过
	err = engine.Approve(ctx, inst.ID, "manager")
	require.NoError(t, err)

	// 继续执行
	err = engine.Execute(ctx, inst.ID)
	require.NoError(t, err)

	result, err = engine.GetInstance(ctx, inst.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, result.Status)
	assert.Equal(t, "manager", result.Data["_approver"])
	assert.Equal(t, true, result.Data["_approved"])
}

func TestApprovalReject(t *testing.T) {
	store := NewMemoryStore()
	engine := New(store)
	ctx := context.Background()

	def := &Definition{
		ID:          "approval-reject",
		Name:        "审批拒绝流程",
		Version:     "1.0",
		StartNodeID: "submit",
		Nodes: map[string]*Node{
			"submit": {
				ID:        "submit",
				Name:      "提交申请",
				Type:      NodeTypeTask,
				NextNodes: []string{"approve"},
			},
			"approve": {
				ID:        "approve",
				Name:      "审批",
				Type:      NodeTypeApproval,
				NextNodes: []string{"end"},
			},
			"end": {
				ID:   "end",
				Name: "结束",
				Type: NodeTypeEnd,
			},
		},
	}

	err := engine.RegisterDefinition(def)
	require.NoError(t, err)

	inst, err := engine.StartWorkflow(ctx, "approval-reject", nil)
	require.NoError(t, err)

	err = engine.Execute(ctx, inst.ID)
	require.NoError(t, err)

	// 审批拒绝
	err = engine.Reject(ctx, inst.ID, "manager", "预算超标")
	require.NoError(t, err)

	result, err := engine.GetInstance(ctx, inst.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Equal(t, false, result.Data["_approved"])
	assert.Equal(t, "预算超标", result.Data["_reject_reason"])
}

func TestConditionBranching(t *testing.T) {
	store := NewMemoryStore()
	engine := New(store)
	ctx := context.Background()

	def := &Definition{
		ID:          "condition-1",
		Name:        "条件分支",
		Version:     "1.0",
		StartNodeID: "check",
		Nodes: map[string]*Node{
			"check": {
				ID:   "check",
				Name: "条件检查",
				Type: NodeTypeCondition,
				Condition: func(_ context.Context, inst *Instance) (string, error) {
					amount, _ := inst.Data["amount"].(float64)
					if amount > 1000 {
						return "high", nil
					}
					return "low", nil
				},
			},
			"high": {
				ID:   "high",
				Name: "高额处理",
				Type: NodeTypeTask,
				Handler: func(_ context.Context, inst *Instance) error {
					inst.Data["path"] = "high"
					return nil
				},
				NextNodes: []string{"end"},
			},
			"low": {
				ID:   "low",
				Name: "低额处理",
				Type: NodeTypeTask,
				Handler: func(_ context.Context, inst *Instance) error {
					inst.Data["path"] = "low"
					return nil
				},
				NextNodes: []string{"end"},
			},
			"end": {
				ID:   "end",
				Name: "结束",
				Type: NodeTypeEnd,
			},
		},
	}

	err := engine.RegisterDefinition(def)
	require.NoError(t, err)

	// 测试高额路径
	inst, err := engine.StartWorkflow(ctx, "condition-1", map[string]any{"amount": float64(2000)})
	require.NoError(t, err)

	err = engine.Execute(ctx, inst.ID)
	require.NoError(t, err)

	result, err := engine.GetInstance(ctx, inst.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, result.Status)
	assert.Equal(t, "high", result.Data["path"])

	// 测试低额路径
	inst2, err := engine.StartWorkflow(ctx, "condition-1", map[string]any{"amount": float64(500)})
	require.NoError(t, err)

	err = engine.Execute(ctx, inst2.ID)
	require.NoError(t, err)

	result2, err := engine.GetInstance(ctx, inst2.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, result2.Status)
	assert.Equal(t, "low", result2.Data["path"])
}

func TestParallelExecution(t *testing.T) {
	store := NewMemoryStore()
	engine := New(store, WithMaxParallel(3))
	ctx := context.Background()

	executed := false
	def := &Definition{
		ID:          "parallel-1",
		Name:        "并行流程",
		Version:     "1.0",
		StartNodeID: "parallel",
		Nodes: map[string]*Node{
			"parallel": {
				ID:   "parallel",
				Name: "并行任务",
				Type: NodeTypeParallel,
				Handler: func(_ context.Context, inst *Instance) error {
					executed = true
					inst.Data["parallel_done"] = true
					return nil
				},
				NextNodes: []string{"end"},
			},
			"end": {
				ID:   "end",
				Name: "结束",
				Type: NodeTypeEnd,
			},
		},
	}

	err := engine.RegisterDefinition(def)
	require.NoError(t, err)

	inst, err := engine.StartWorkflow(ctx, "parallel-1", nil)
	require.NoError(t, err)

	err = engine.Execute(ctx, inst.ID)
	require.NoError(t, err)

	assert.True(t, executed)
	result, err := engine.GetInstance(ctx, inst.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, result.Status)
}

func TestCancel(t *testing.T) {
	store := NewMemoryStore()
	engine := New(store)
	ctx := context.Background()

	def := &Definition{
		ID:          "cancel-1",
		Name:        "可取消流程",
		Version:     "1.0",
		StartNodeID: "step1",
		Nodes: map[string]*Node{
			"step1": {
				ID:        "step1",
				Name:      "步骤一",
				Type:      NodeTypeApproval,
				NextNodes: []string{"end"},
			},
			"end": {
				ID:   "end",
				Name: "结束",
				Type: NodeTypeEnd,
			},
		},
	}

	err := engine.RegisterDefinition(def)
	require.NoError(t, err)

	inst, err := engine.StartWorkflow(ctx, "cancel-1", nil)
	require.NoError(t, err)

	// 取消工作流
	err = engine.Cancel(ctx, inst.ID)
	require.NoError(t, err)

	result, err := engine.GetInstance(ctx, inst.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusCancelled, result.Status)

	// 已取消的工作流不能再执行
	err = engine.Execute(ctx, inst.ID)
	assert.ErrorIs(t, err, ErrWorkflowCompleted)
}

func TestDefinitionNotFound(t *testing.T) {
	store := NewMemoryStore()
	engine := New(store)
	ctx := context.Background()

	_, err := engine.StartWorkflow(ctx, "nonexistent", nil)
	assert.ErrorIs(t, err, ErrDefinitionNotFound)
}

func TestInstanceNotFound(t *testing.T) {
	store := NewMemoryStore()
	engine := New(store)
	ctx := context.Background()

	err := engine.Execute(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrInstanceNotFound)
}

func TestRegisterInvalidDefinition(t *testing.T) {
	store := NewMemoryStore()
	engine := New(store)

	// 缺少 ID
	err := engine.RegisterDefinition(&Definition{StartNodeID: "start", Nodes: map[string]*Node{}})
	assert.Error(t, err)

	// StartNodeID 不在 Nodes 中
	err = engine.RegisterDefinition(&Definition{
		ID:          "bad",
		StartNodeID: "missing",
		Nodes:       map[string]*Node{"start": {ID: "start"}},
	})
	assert.Error(t, err)
}

func TestHandlerError(t *testing.T) {
	store := NewMemoryStore()
	engine := New(store)
	ctx := context.Background()

	handlerErr := errors.New("handler failed")
	def := &Definition{
		ID:          "err-1",
		Name:        "错误流程",
		Version:     "1.0",
		StartNodeID: "fail",
		Nodes: map[string]*Node{
			"fail": {
				ID:   "fail",
				Name: "失败任务",
				Type: NodeTypeTask,
				Handler: func(_ context.Context, _ *Instance) error {
					return handlerErr
				},
				NextNodes: []string{"end"},
			},
			"end": {
				ID:   "end",
				Name: "结束",
				Type: NodeTypeEnd,
			},
		},
	}

	err := engine.RegisterDefinition(def)
	require.NoError(t, err)

	inst, err := engine.StartWorkflow(ctx, "err-1", nil)
	require.NoError(t, err)

	err = engine.Execute(ctx, inst.ID)
	assert.ErrorIs(t, err, handlerErr)

	result, err := engine.GetInstance(ctx, inst.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, result.Status)
}
