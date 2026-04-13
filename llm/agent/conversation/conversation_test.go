package conversation_test

import (
	"context"
	"io"
	"testing"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/agent/conversation"
)

// mockModel 模拟 ChatModel.
type mockModel struct {
	responses []string
	idx       int
}

func (m *mockModel) Generate(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	if m.idx >= len(m.responses) {
		return &llm.ChatResponse{Message: llm.AssistantMessage("default")}, nil
	}
	resp := m.responses[m.idx]
	m.idx++
	return &llm.ChatResponse{Message: llm.AssistantMessage(resp)}, nil
}

func (m *mockModel) Stream(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
	return &mockReader{content: "stream"}, nil
}

// mockReader 模拟 StreamReader.
type mockReader struct {
	content string
	sent    bool
}

func (r *mockReader) Recv() (llm.StreamChunk, error) {
	if r.sent {
		return llm.StreamChunk{}, io.EOF
	}
	r.sent = true
	return llm.StreamChunk{Delta: r.content, FinishReason: "stop"}, nil
}

func (r *mockReader) Response() *llm.ChatResponse {
	return &llm.ChatResponse{Message: llm.AssistantMessage(r.content)}
}

func (r *mockReader) Close() error { return nil }

func TestConversation_Chat(t *testing.T) {
	model := &mockModel{responses: []string{"你好！", "我很好，谢谢！"}}
	conv := conversation.New(model)

	resp1, err := conv.Chat(t.Context(), "你好")
	if err != nil {
		t.Fatalf("第一次 Chat 失败: %v", err)
	}
	if resp1.Message.Content != "你好！" {
		t.Errorf("期望 '你好！'，得到 %q", resp1.Message.Content)
	}

	resp2, err := conv.Chat(t.Context(), "你好吗？")
	if err != nil {
		t.Fatalf("第二次 Chat 失败: %v", err)
	}
	if resp2.Message.Content != "我很好，谢谢！" {
		t.Errorf("期望 '我很好，谢谢！'，得到 %q", resp2.Message.Content)
	}

	// 验证历史保留
	history := conv.History()
	if len(history) != 4 {
		t.Errorf("期望 4 条历史消息（2 用户 + 2 助手），得到 %d", len(history))
	}
}

func TestConversation_WithSystemPrompt(t *testing.T) {
	var capturedMessages []llm.Message
	model := &mockCaptureModel{capture: &capturedMessages}
	conv := conversation.New(model, conversation.WithSystemPrompt("你是一个助手"))

	_, _ = conv.Chat(t.Context(), "hi")

	if len(capturedMessages) == 0 {
		t.Fatal("未捕获消息")
	}
	if capturedMessages[0].Role != llm.RoleSystem {
		t.Errorf("期望第一条消息为 system，得到 %s", capturedMessages[0].Role)
	}
	if capturedMessages[0].Content != "你是一个助手" {
		t.Errorf("期望系统提示 '你是一个助手'，得到 %q", capturedMessages[0].Content)
	}
}

func TestConversation_Reset(t *testing.T) {
	model := &mockModel{responses: []string{"ok"}}
	conv := conversation.New(model)

	_, _ = conv.Chat(t.Context(), "hi")
	if len(conv.History()) == 0 {
		t.Fatal("期望有历史消息")
	}

	conv.Reset()
	if len(conv.History()) != 0 {
		t.Errorf("Reset 后期望历史为空，得到 %d 条", len(conv.History()))
	}
}

// mockCaptureModel 捕获发送消息的模型.
type mockCaptureModel struct {
	capture *[]llm.Message
}

func (m *mockCaptureModel) Generate(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	*m.capture = append(*m.capture, messages...)
	return &llm.ChatResponse{Message: llm.AssistantMessage("ok")}, nil
}

func (m *mockCaptureModel) Stream(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
	return &mockReader{content: "ok"}, nil
}

func TestBufferMemory(t *testing.T) {
	mem := conversation.NewBufferMemory()
	mem.Add(llm.UserMessage("hi"))
	mem.Add(llm.AssistantMessage("hello"))

	msgs := mem.Messages()
	if len(msgs) != 2 {
		t.Errorf("期望 2 条消息，得到 %d", len(msgs))
	}

	mem.Clear()
	if len(mem.Messages()) != 0 {
		t.Error("Clear 后期望为空")
	}
}

func TestWindowMemory_Trim(t *testing.T) {
	// 最大 2 轮 = 4 条消息
	mem := conversation.NewWindowMemory(2)

	for range 3 {
		mem.Add(llm.UserMessage("user"))
		mem.Add(llm.AssistantMessage("assistant"))
	}

	msgs := mem.Messages()
	// 只保留最近 2 轮（4 条）
	if len(msgs) != 4 {
		t.Errorf("期望 4 条消息（2 轮），得到 %d", len(msgs))
	}
}

// --- ChatStream ---

func TestConversation_ChatStream(t *testing.T) {
	model := &mockModel{responses: []string{"streamed"}}
	conv := conversation.New(model)

	reader, err := conv.ChatStream(t.Context(), "hello stream")
	if err != nil {
		t.Fatalf("ChatStream error: %v", err)
	}
	defer reader.Close()

	// Read first chunk.
	chunk, err := reader.Recv()
	if err != nil {
		t.Fatalf("Recv error: %v", err)
	}
	if chunk.Delta != "stream" {
		t.Errorf("expected delta 'stream', got %q", chunk.Delta)
	}

	// Read EOF, which triggers auto-record.
	_, err = reader.Recv()
	if err != io.EOF {
		t.Fatalf("expected io.EOF, got %v", err)
	}

	// Verify response available.
	resp := reader.Response()
	if resp == nil {
		t.Fatal("Response should not be nil after stream ends")
	}
}

// --- WithMemory option ---

func TestConversation_WithMemory(t *testing.T) {
	model := &mockModel{responses: []string{"r1", "r2", "r3"}}
	mem := conversation.NewWindowMemory(1) // 1 round = 2 messages max
	conv := conversation.New(model, conversation.WithMemory(mem))

	_, _ = conv.Chat(t.Context(), "q1")
	_, _ = conv.Chat(t.Context(), "q2")

	// WindowMemory(1) keeps only last round (2 messages).
	history := conv.History()
	if len(history) != 2 {
		t.Errorf("expected 2 messages in window, got %d", len(history))
	}
}

// --- WindowMemory edge cases ---

func TestWindowMemory_MinRounds(t *testing.T) {
	// maxRounds < 1 should be clamped to 1.
	mem := conversation.NewWindowMemory(0)
	mem.Add(llm.UserMessage("u1"))
	mem.Add(llm.AssistantMessage("a1"))
	mem.Add(llm.UserMessage("u2"))
	mem.Add(llm.AssistantMessage("a2"))

	msgs := mem.Messages()
	// maxRounds=1 => 2 messages max
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages (1 round), got %d", len(msgs))
	}
}

func TestWindowMemory_NegativeRounds(t *testing.T) {
	mem := conversation.NewWindowMemory(-5)
	mem.Add(llm.UserMessage("u"))
	mem.Add(llm.AssistantMessage("a"))
	mem.Add(llm.UserMessage("u2"))

	msgs := mem.Messages()
	if len(msgs) > 2 {
		t.Errorf("expected at most 2 messages, got %d", len(msgs))
	}
}

func TestWindowMemory_Clear(t *testing.T) {
	mem := conversation.NewWindowMemory(5)
	mem.Add(llm.UserMessage("u"))
	mem.Clear()
	if len(mem.Messages()) != 0 {
		t.Error("expected empty after Clear")
	}
}

// --- BufferMemory returns copy ---

func TestBufferMemory_ReturnsCopy(t *testing.T) {
	mem := conversation.NewBufferMemory()
	mem.Add(llm.UserMessage("original"))

	msgs := mem.Messages()
	msgs[0] = llm.AssistantMessage("modified")

	// Original should be unchanged.
	if mem.Messages()[0].Role != llm.RoleUser {
		t.Error("Messages() should return a copy")
	}
}

// --- Conversation with no system prompt ---

func TestConversation_NoSystemPrompt(t *testing.T) {
	var capturedMessages []llm.Message
	model := &mockCaptureModel{capture: &capturedMessages}
	conv := conversation.New(model)

	_, _ = conv.Chat(t.Context(), "hi")

	// Without system prompt, first message should be user.
	if len(capturedMessages) == 0 {
		t.Fatal("no messages captured")
	}
	if capturedMessages[0].Role != llm.RoleUser {
		t.Errorf("expected first message to be user, got %s", capturedMessages[0].Role)
	}
}
