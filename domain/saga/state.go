package saga

import (
	"time"
)

// State Saga 执行状态.
type State struct {
	// ID Saga 唯一标识
	ID string `json:"id"`

	// Name Saga 名称
	Name string `json:"name"`

	// Status 当前状态
	Status SagaStatus `json:"status"`

	// CurrentStep 当前执行的步骤索引
	CurrentStep int `json:"current_step"`

	// StepResults 各步骤执行结果
	StepResults []StepResult `json:"step_results"`

	// Error 错误信息
	Error string `json:"error,omitempty"`

	// StartedAt 开始时间
	StartedAt time.Time `json:"started_at"`

	// CompletedAt 完成时间
	CompletedAt *time.Time `json:"completed_at,omitzero"`

	// Data 共享数据序列化后
	Data map[string]any `json:"data,omitzero"`
}

// SagaStatus Saga 状态.
type SagaStatus string

const (
	// SagaStatusPending 待执行.
	SagaStatusPending SagaStatus = "pending"

	// SagaStatusRunning 执行中.
	SagaStatusRunning SagaStatus = "running"

	// SagaStatusCompleted 执行完成.
	SagaStatusCompleted SagaStatus = "completed"

	// SagaStatusFailed 执行失败.
	SagaStatusFailed SagaStatus = "failed"

	// SagaStatusCompensating 补偿中.
	SagaStatusCompensating SagaStatus = "compensating"

	// SagaStatusCompensated 已补偿.
	SagaStatusCompensated SagaStatus = "compensated"

	// SagaStatusCompensateFailed 补偿失败.
	SagaStatusCompensateFailed SagaStatus = "compensate_failed"
)

// IsTerminal 是否为终态.
func (s SagaStatus) IsTerminal() bool {
	switch s {
	case SagaStatusCompleted, SagaStatusCompensated, SagaStatusCompensateFailed:
		return true
	default:
		return false
	}
}

// NewState 创建新的 Saga 状态.
func NewState(id, name string, stepCount int) *State {
	results := make([]StepResult, stepCount)
	for i := range results {
		results[i] = StepResult{
			Status: StepStatusPending,
		}
	}

	return &State{
		ID:          id,
		Name:        name,
		Status:      SagaStatusPending,
		CurrentStep: 0,
		StepResults: results,
		StartedAt:   time.Now(),
		Data:        make(map[string]any),
	}
}
