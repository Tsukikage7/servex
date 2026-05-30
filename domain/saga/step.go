package saga

import (
	"context"
)

// StepFunc 步骤执行函数类型.
// ctx: 执行上下文
// data: Saga 共享数据，可以在步骤间传递数据
// 返回错误表示步骤失败，将触发补偿.
type StepFunc func(ctx context.Context, data *Data) error

// CompensateFunc 补偿函数类型.
// 当步骤失败时，会按逆序调用已执行步骤的补偿函数.
// 补偿函数应该是幂等的，因为可能被多次调用.
type CompensateFunc func(ctx context.Context, data *Data) error

// Step 表示 Saga 中的一个步骤.
type Step struct {
	// Name 步骤名称
	Name string

	// Action 正向操作
	Action StepFunc

	// Compensate 补偿操作可选
	// 如果为 nil，表示该步骤不需要补偿
	Compensate CompensateFunc
}

// StepResult 步骤执行结果.
type StepResult struct {
	// StepName 步骤名称
	StepName string

	// Status 执行状态
	Status StepStatus

	// Error 错误信息如果有
	Error error

	// Duration 执行耗时
	Duration int64 // 毫秒
}

// StepStatus 步骤状态.
type StepStatus string

const (
	// StepStatusPending 待执行.
	StepStatusPending StepStatus = "pending"

	// StepStatusRunning 执行中.
	StepStatusRunning StepStatus = "running"

	// StepStatusCompleted 执行完成.
	StepStatusCompleted StepStatus = "completed"

	// StepStatusFailed 执行失败.
	StepStatusFailed StepStatus = "failed"

	// StepStatusCompensating 补偿中.
	StepStatusCompensating StepStatus = "compensating"

	// StepStatusCompensated 已补偿.
	StepStatusCompensated StepStatus = "compensated"

	// StepStatusCompensateFailed 补偿失败.
	StepStatusCompensateFailed StepStatus = "compensate_failed"
)

// Data Saga 共享数据.
// 用于在步骤之间传递数据.
type Data struct {
	values map[string]any
}

// NewData 创建新的共享数据.
func NewData() *Data {
	return &Data{
		values: make(map[string]any),
	}
}

// Set 设置数据.
func (d *Data) Set(key string, value any) {
	d.values[key] = value
}

// Get 获取数据.
func (d *Data) Get(key string) (any, bool) {
	v, ok := d.values[key]
	return v, ok
}

// GetString 获取字符串数据.
func (d *Data) GetString(key string) string {
	v, ok := d.values[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// GetInt 获取整数数据.
func (d *Data) GetInt(key string) int {
	v, ok := d.values[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case int32:
		return int(n)
	default:
		return 0
	}
}

// GetInt64 获取 int64 数据.
func (d *Data) GetInt64(key string) int64 {
	v, ok := d.values[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	default:
		return 0
	}
}

// GetBool 获取布尔数据.
func (d *Data) GetBool(key string) bool {
	v, ok := d.values[key]
	if !ok {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// Delete 删除数据.
func (d *Data) Delete(key string) {
	delete(d.values, key)
}

// Keys 返回所有键.
func (d *Data) Keys() []string {
	keys := make([]string, 0, len(d.values))
	for k := range d.values {
		keys = append(keys, k)
	}
	return keys
}
