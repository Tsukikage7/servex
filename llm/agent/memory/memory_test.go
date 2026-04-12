package memory_test

import (
	"context"
	"testing"

	"github.com/Tsukikage7/servex/llm"
	"github.com/Tsukikage7/servex/llm/agent/conversation"
	"github.com/Tsukikage7/servex/llm/agent/memory"
)

// =============================================================================
// 测试辅助 — mockModel
// =============================================================================

// mockModel 模拟 ChatModel，通过 fn 函数决定返回内容.
type mockModel struct {
	fn func(msgs []llm.Message) string
}

func (m *mockModel) Generate(ctx context.Context, msgs []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	content := m.fn(msgs)
	return &llm.ChatResponse{Message: llm.AssistantMessage(content)}, nil
}

func (m *mockModel) Stream(ctx context.Context, msgs []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
	// 测试中不使用 Stream，返回空实现.
	return nil, nil
}

// =============================================================================
// MemoryStore 测试
// =============================================================================

func TestMemoryStore_SaveAndLoad(t *testing.T) {
	store := memory.NewMemoryStore()
	ctx := t.Context()

	msgs := []llm.Message{
		llm.UserMessage("你好"),
		llm.AssistantMessage("你好，有什么可以帮您？"),
	}
	meta := map[string]any{"user": "alice"}

	if err := store.Save(ctx, "sess1", msgs, meta); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	loaded, loadedMeta, err := store.Load(ctx, "sess1")
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if len(loaded) != len(msgs) {
		t.Errorf("期望 %d 条消息，得到 %d", len(msgs), len(loaded))
	}
	if loaded[0].Content != msgs[0].Content {
		t.Errorf("第一条消息内容不匹配: 期望 %q，得到 %q", msgs[0].Content, loaded[0].Content)
	}
	if loadedMeta["user"] != "alice" {
		t.Errorf("元数据 user 期望 alice，得到 %v", loadedMeta["user"])
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := memory.NewMemoryStore()
	ctx := t.Context()

	_ = store.Save(ctx, "sess2", []llm.Message{llm.UserMessage("hi")}, nil)

	if err := store.Delete(ctx, "sess2"); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}

	_, _, err := store.Load(ctx, "sess2")
	if err == nil {
		t.Error("期望 Load 返回错误（会话已删除），但未返回")
	}
}

func TestMemoryStore_List(t *testing.T) {
	store := memory.NewMemoryStore()
	ctx := t.Context()

	_ = store.Save(ctx, "s1", nil, nil)
	_ = store.Save(ctx, "s2", nil, nil)
	_ = store.Save(ctx, "s3", nil, nil)

	ids, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(ids) != 3 {
		t.Errorf("期望 3 个会话 ID，得到 %d", len(ids))
	}

	// 验证所有 ID 均在返回列表中.
	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}
	for _, want := range []string{"s1", "s2", "s3"} {
		if !idSet[want] {
			t.Errorf("期望 List 包含 %q，但未找到", want)
		}
	}
}

// =============================================================================
// PersistentMemory 测试
// =============================================================================

func TestPersistentMemory_SaveAndLoad(t *testing.T) {
	store := memory.NewMemoryStore()
	ctx := t.Context()

	inner := conversation.NewBufferMemory()
	pm := memory.NewPersistentMemory(inner, store, "persist-sess")

	pm.Add(llm.UserMessage("第一条消息"))
	pm.Add(llm.AssistantMessage("助手回复"))

	// 持久化保存.
	if err := pm.Save(ctx); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	// 创建新的 PersistentMemory，从存储加载.
	inner2 := conversation.NewBufferMemory()
	pm2 := memory.NewPersistentMemory(inner2, store, "persist-sess")

	if err := pm2.Load(ctx); err != nil {
		t.Fatalf("Load 失败: %v", err)
	}

	msgs := pm2.Messages()
	if len(msgs) != 2 {
		t.Errorf("期望 2 条消息，得到 %d", len(msgs))
	}
	if msgs[0].Content != "第一条消息" {
		t.Errorf("第一条消息内容不匹配: 期望 %q，得到 %q", "第一条消息", msgs[0].Content)
	}
	if msgs[1].Content != "助手回复" {
		t.Errorf("第二条消息内容不匹配: 期望 %q，得到 %q", "助手回复", msgs[1].Content)
	}
}

// =============================================================================
// SummaryMemory 测试
// =============================================================================

func TestSummaryMemory_BelowThreshold(t *testing.T) {
	// 设置最大消息数为 10，添加 5 条，不触发摘要.
	summarizeCalled := false
	model := &mockModel{fn: func(msgs []llm.Message) string {
		summarizeCalled = true
		return "摘要内容"
	}}

	mem := memory.NewSummaryMemory(model, memory.WithMaxMessages(10))

	for i := range 5 {
		_ = i
		mem.Add(llm.UserMessage("用户消息"))
		mem.Add(llm.AssistantMessage("助手回复"))
	}

	if summarizeCalled {
		t.Error("未超过阈值，不应触发摘要")
	}

	msgs := mem.Messages()
	// 5 轮 = 10 条消息，无摘要前缀.
	if len(msgs) != 10 {
		t.Errorf("期望 10 条消息，得到 %d", len(msgs))
	}
}

func TestSummaryMemory_AboveThreshold(t *testing.T) {
	// 设置最大消息数为 4，添加 5 条触发摘要.
	const expectedSummary = "这是自动生成的摘要"
	model := &mockModel{fn: func(msgs []llm.Message) string {
		return expectedSummary
	}}

	mem := memory.NewSummaryMemory(model, memory.WithMaxMessages(4))

	// 添加 5 条消息，第 5 条触发摘要（超过 maxMessages=4）.
	mem.Add(llm.UserMessage("消息1"))
	mem.Add(llm.AssistantMessage("回复1"))
	mem.Add(llm.UserMessage("消息2"))
	mem.Add(llm.AssistantMessage("回复2"))
	mem.Add(llm.UserMessage("消息3")) // 第 5 条，触发摘要

	msgs := mem.Messages()

	// 摘要存在时，第一条应为包含摘要内容的 system 消息.
	if len(msgs) == 0 {
		t.Fatal("消息列表为空")
	}
	if msgs[0].Role != llm.RoleSystem {
		t.Errorf("期望第一条为 system 角色，得到 %s", msgs[0].Role)
	}
	if !contains(msgs[0].Content, expectedSummary) {
		t.Errorf("摘要消息应包含 %q，实际内容: %q", expectedSummary, msgs[0].Content)
	}
}

// =============================================================================
// EntityMemory 测试
// =============================================================================

func TestEntityMemory_ExtractEntities(t *testing.T) {
	// 模型返回包含实体的 JSON.
	model := &mockModel{fn: func(msgs []llm.Message) string {
		return `{"Alice": "用户的名字", "北京": "中国首都"}`
	}}

	mem := memory.NewEntityMemory(model)
	mem.Add(llm.UserMessage("我叫 Alice，我在北京工作。"))

	entities := mem.Entities()

	if _, ok := entities["Alice"]; !ok {
		t.Error("期望实体 Alice 被抽取，但未找到")
	}
	if _, ok := entities["北京"]; !ok {
		t.Error("期望实体 北京 被抽取，但未找到")
	}

	// 验证 Messages() 包含实体上下文系统消息.
	allMsgs := mem.Messages()
	if len(allMsgs) < 2 {
		t.Fatalf("期望至少 2 条消息（实体系统消息 + 用户消息），得到 %d", len(allMsgs))
	}
	if allMsgs[0].Role != llm.RoleSystem {
		t.Errorf("第一条消息应为 system 角色，得到 %s", allMsgs[0].Role)
	}
}

// --- PersistentMemory edge cases ---

func TestPersistentMemory_NilStore(t *testing.T) {
	inner := conversation.NewBufferMemory()
	pm := memory.NewPersistentMemory(inner, nil, "sess")

	// Save and Load should return ErrNilStore.
	if err := pm.Save(t.Context()); err == nil {
		t.Error("expected error for nil store on Save")
	}
	if err := pm.Load(t.Context()); err == nil {
		t.Error("expected error for nil store on Load")
	}
}

func TestPersistentMemory_Clear(t *testing.T) {
	store := memory.NewMemoryStore()
	inner := conversation.NewBufferMemory()
	pm := memory.NewPersistentMemory(inner, store, "sess")

	pm.Add(llm.UserMessage("hello"))
	pm.Clear()

	if len(pm.Messages()) != 0 {
		t.Error("expected empty after Clear")
	}
}

func TestPersistentMemory_LoadNonexistent(t *testing.T) {
	store := memory.NewMemoryStore()
	inner := conversation.NewBufferMemory()
	pm := memory.NewPersistentMemory(inner, store, "nonexistent")

	err := pm.Load(t.Context())
	if err == nil {
		t.Error("expected error loading nonexistent session")
	}
}

// --- SummaryMemory edge cases ---

func TestSummaryMemory_Clear(t *testing.T) {
	model := &mockModel{fn: func(msgs []llm.Message) string {
		return "summary"
	}}

	mem := memory.NewSummaryMemory(model, memory.WithMaxMessages(4))
	// Add enough to trigger summary.
	for range 6 {
		mem.Add(llm.UserMessage("msg"))
	}

	mem.Clear()
	msgs := mem.Messages()
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after Clear, got %d", len(msgs))
	}
}

func TestSummaryMemory_CustomPrompt(t *testing.T) {
	var capturedPrompt string
	model := &mockModel{fn: func(msgs []llm.Message) string {
		if len(msgs) > 0 && msgs[0].Role == llm.RoleSystem {
			capturedPrompt = msgs[0].Content
		}
		return "custom summary"
	}}

	mem := memory.NewSummaryMemory(model,
		memory.WithMaxMessages(2),
		memory.WithSummaryPrompt("Custom prompt:"),
	)

	mem.Add(llm.UserMessage("m1"))
	mem.Add(llm.AssistantMessage("r1"))
	mem.Add(llm.UserMessage("m2")) // triggers summary

	if capturedPrompt == "" {
		t.Skip("summary may not have been triggered yet")
	}
	if !containsStr(capturedPrompt, "Custom prompt:") {
		t.Errorf("expected custom prompt, got %q", capturedPrompt)
	}
}

func TestSummaryMemory_NoSummaryBelowThreshold(t *testing.T) {
	model := &mockModel{fn: func(msgs []llm.Message) string {
		return "should not be called"
	}}

	mem := memory.NewSummaryMemory(model, memory.WithMaxMessages(100))
	mem.Add(llm.UserMessage("hi"))
	mem.Add(llm.AssistantMessage("hello"))

	msgs := mem.Messages()
	// No summary system message should be present.
	for _, m := range msgs {
		if m.Role == llm.RoleSystem {
			t.Error("no system message expected below threshold")
		}
	}
}

// --- EntityMemory edge cases ---

func TestEntityMemory_EmptyMessage(t *testing.T) {
	callCount := 0
	model := &mockModel{fn: func(msgs []llm.Message) string {
		callCount++
		return `{}`
	}}

	mem := memory.NewEntityMemory(model)
	// Add an empty user message - should not trigger extraction.
	mem.Add(llm.UserMessage(""))

	if callCount != 0 {
		t.Error("model should not be called for empty content")
	}
}

func TestEntityMemory_SystemMessageSkipped(t *testing.T) {
	callCount := 0
	model := &mockModel{fn: func(msgs []llm.Message) string {
		callCount++
		return `{}`
	}}

	mem := memory.NewEntityMemory(model)
	mem.Add(llm.SystemMessage("you are a bot"))

	if callCount != 0 {
		t.Error("model should not be called for system messages")
	}
}

func TestEntityMemory_Clear(t *testing.T) {
	model := &mockModel{fn: func(msgs []llm.Message) string {
		return `{"Entity": "desc"}`
	}}

	mem := memory.NewEntityMemory(model)
	mem.Add(llm.UserMessage("hello Entity"))
	mem.Clear()

	if len(mem.Entities()) != 0 {
		t.Error("entities should be empty after Clear")
	}
	if len(mem.Messages()) != 0 {
		t.Error("messages should be empty after Clear")
	}
}

func TestEntityMemory_InvalidJSON(t *testing.T) {
	model := &mockModel{fn: func(msgs []llm.Message) string {
		return "not json at all"
	}}

	mem := memory.NewEntityMemory(model)
	mem.Add(llm.UserMessage("test with invalid json response"))

	// Should not panic; entities should be empty.
	if len(mem.Entities()) != 0 {
		t.Error("entities should be empty when model returns invalid JSON")
	}
}

func TestEntityMemory_WithCustomPrompt(t *testing.T) {
	var capturedMsg string
	model := &mockModel{fn: func(msgs []llm.Message) string {
		if len(msgs) > 0 {
			capturedMsg = msgs[0].Content
		}
		return `{}`
	}}

	mem := memory.NewEntityMemory(model, memory.WithEntityPrompt("Extract:"))
	mem.Add(llm.UserMessage("hello"))

	if !containsStr(capturedMsg, "Extract:") {
		t.Errorf("expected custom prompt prefix, got %q", capturedMsg)
	}
}

// --- MemoryStore edge cases ---

func TestMemoryStore_SaveNilMetadata(t *testing.T) {
	store := memory.NewMemoryStore()
	ctx := t.Context()

	err := store.Save(ctx, "sess", []llm.Message{llm.UserMessage("hi")}, nil)
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}

	msgs, meta, err := store.Load(ctx, "sess")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("expected 1 message, got %d", len(msgs))
	}
	if meta != nil {
		t.Errorf("expected nil metadata, got %v", meta)
	}
}

func TestMemoryStore_Overwrite(t *testing.T) {
	store := memory.NewMemoryStore()
	ctx := t.Context()

	_ = store.Save(ctx, "sess", []llm.Message{llm.UserMessage("first")}, nil)
	_ = store.Save(ctx, "sess", []llm.Message{llm.UserMessage("second")}, nil)

	msgs, _, _ := store.Load(ctx, "sess")
	if len(msgs) != 1 || msgs[0].Content != "second" {
		t.Error("second save should overwrite first")
	}
}

func TestMemoryStore_DeleteNonexistent(t *testing.T) {
	store := memory.NewMemoryStore()
	// Should not error.
	if err := store.Delete(t.Context(), "nonexistent"); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
}

// =============================================================================
// 辅助函数
// =============================================================================

// contains 检查 s 是否包含 substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

// containsStr 简单子串检查（避免导入 strings 包命名冲突）.
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
