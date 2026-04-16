package agent

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/agent/checkpoint"
)

// InterruptPolicy 中断策略接口.
type InterruptPolicy interface {
	ShouldInterrupt(ctx context.Context, toolCall llm.ToolCall) bool
}

// AlwaysInterruptPolicy 所有工具调用都中断.
type AlwaysInterruptPolicy struct{}

// ShouldInterrupt 永远返回 true，所有工具调用都触发中断.
func (AlwaysInterruptPolicy) ShouldInterrupt(_ context.Context, _ llm.ToolCall) bool { return true }

// ToolNamePolicy 指定工具名中断.
type ToolNamePolicy struct{ Names []string }

// ShouldInterrupt 当工具调用名称在 Names 列表中时返回 true.
func (p ToolNamePolicy) ShouldInterrupt(_ context.Context, tc llm.ToolCall) bool {
	for _, name := range p.Names {
		if tc.Function.Name == name {
			return true
		}
	}
	return false
}

// InterruptError 中断错误，包含检查点信息.
type InterruptError struct {
	CheckpointID string
	ToolCalls    []llm.ToolCall
	Messages     []llm.Message
}

// Error 实现 error 接口.
func (e *InterruptError) Error() string {
	return fmt.Sprintf("agent: interrupted, checkpoint=%s, tools=%d", e.CheckpointID, len(e.ToolCalls))
}

// ToolApproval 工具调用审批结果.
type ToolApproval struct {
	Approved bool   `json:"approved"`
	Output   string `json:"output,omitempty"`  // Approved=false 时的替代输出
	Reason   string `json:"reason,omitempty"`  // 审批原因（审计用）
}

// ---------- context key ----------

// interruptConfigKey context 中断配置 key.
type interruptConfigKey struct{}

// interruptConfig 通过 context 传递的中断配置.
type interruptConfig struct {
	policy    InterruptPolicy
	store     checkpoint.Store
	agentName string
}

// withInterruptConfig 将中断配置注入 context.
func withInterruptConfig(ctx context.Context, cfg *interruptConfig) context.Context {
	return context.WithValue(ctx, interruptConfigKey{}, cfg)
}

// getInterruptConfig 从 context 中取出中断配置，不存在时返回 nil.
func getInterruptConfig(ctx context.Context) *interruptConfig {
	v, _ := ctx.Value(interruptConfigKey{}).(*interruptConfig)
	return v
}
