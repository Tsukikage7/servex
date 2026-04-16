// Package checkpoint 提供 Agent 执行快照的存储与恢复能力.
package checkpoint

import (
	"context"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
)

// AgentCheckpoint 代理执行快照.
type AgentCheckpoint struct {
	ID        string         `json:"id"`
	Messages  []llm.Message  `json:"messages"`
	ToolCalls []llm.ToolCall `json:"tool_calls"`
	Iteration int            `json:"iteration"`
	CreatedAt time.Time      `json:"created_at"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// Store 检查点存储接口.
type Store interface {
	Save(ctx context.Context, cp *AgentCheckpoint) error
	Load(ctx context.Context, id string) (*AgentCheckpoint, error)
	Delete(ctx context.Context, id string) error
}
