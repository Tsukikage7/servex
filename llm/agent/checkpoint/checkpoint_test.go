package checkpoint_test

import (
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/agent/checkpoint"
)

func TestMemoryStore_SaveLoadDelete(t *testing.T) {
	store := checkpoint.NewMemoryStore()
	ctx := t.Context()

	cp := &checkpoint.AgentCheckpoint{
		ID: "test-id-1",
		Messages: []llm.Message{
			llm.UserMessage("hello"),
			llm.AssistantMessage("world"),
		},
		ToolCalls: []llm.ToolCall{
			{
				ID:       "call_1",
				Function: struct{ Name, Arguments string }{Name: "my_tool", Arguments: `{}`},
			},
		},
		Iteration: 2,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"agent": "test-agent"},
	}

	// Save
	if err := store.Save(ctx, cp); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	// Load
	loaded, err := store.Load(ctx, cp.ID)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if loaded.ID != cp.ID {
		t.Fatalf("ID 不匹配: 期望 %q, 实际 %q", cp.ID, loaded.ID)
	}
	if len(loaded.Messages) != 2 {
		t.Fatalf("Messages 长度不匹配: 期望 2, 实际 %d", len(loaded.Messages))
	}
	if len(loaded.ToolCalls) != 1 {
		t.Fatalf("ToolCalls 长度不匹配: 期望 1, 实际 %d", len(loaded.ToolCalls))
	}
	if loaded.Iteration != 2 {
		t.Fatalf("Iteration 不匹配: 期望 2, 实际 %d", loaded.Iteration)
	}

	// Delete
	if err := store.Delete(ctx, cp.ID); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}

	// Load after delete should fail
	_, err = store.Load(ctx, cp.ID)
	if err == nil {
		t.Fatal("删除后 Load 应返回错误")
	}
}

func TestMemoryStore_LoadNonExistent(t *testing.T) {
	store := checkpoint.NewMemoryStore()
	_, err := store.Load(t.Context(), "nonexistent-id")
	if err == nil {
		t.Fatal("加载不存在的检查点应返回错误")
	}
}

func TestMemoryStore_DeleteNonExistent(t *testing.T) {
	store := checkpoint.NewMemoryStore()
	// 删除不存在的检查点不应报错
	if err := store.Delete(t.Context(), "nonexistent-id"); err != nil {
		t.Fatalf("删除不存在的检查点不应返回错误: %v", err)
	}
}
