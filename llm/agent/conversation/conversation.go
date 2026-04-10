// Package conversation 提供 AI 对话会话管理功能.
package conversation

import (
	"context"

	"github.com/Tsukikage7/servex/llm"
)

// Option 会话选项.
type Option func(*conversationOptions)

// conversationOptions 内部选项集合.
type conversationOptions struct {
	memory       Memory
	systemPrompt string
}

// WithMemory 设置记忆策略（默认使用 BufferMemory）.
func WithMemory(m Memory) Option {
	return func(o *conversationOptions) { o.memory = m }
}

// WithSystemPrompt 设置系统提示词，每次请求时自动注入.
func WithSystemPrompt(prompt string) Option {
	return func(o *conversationOptions) { o.systemPrompt = prompt }
}

// Conversation 多轮对话会话管理器.
// 自动维护对话历史，每次 Chat 时将历史消息和当前输入一起发送给模型.
type Conversation struct {
	model        llm.ChatModel
	memory       Memory
	systemPrompt string
}

// New 创建会话管理器.
func New(model llm.ChatModel, opts ...Option) *Conversation {
	o := &conversationOptions{
		memory: NewBufferMemory(),
	}
	for _, opt := range opts {
		opt(o)
	}
	return &Conversation{
		model:        model,
		memory:       o.memory,
		systemPrompt: o.systemPrompt,
	}
}

// Chat 发送一条用户消息，返回助手响应.
// 自动将历史消息和当前输入拼接后发送.
func (c *Conversation) Chat(ctx context.Context, input string, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	c.memory.Add(llm.UserMessage(input))
	messages := c.buildMessages()

	resp, err := c.model.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, err
	}

	c.memory.Add(resp.Message)
	return resp, nil
}

// ChatStream 发送一条用户消息，返回流式读取器.
// 注意：流式模式下需调用者在流结束后手动记录助手消息（可通过 reader.Response() 获取）.
func (c *Conversation) ChatStream(ctx context.Context, input string, opts ...llm.CallOption) (llm.StreamReader, error) {
	c.memory.Add(llm.UserMessage(input))
	messages := c.buildMessages()

	reader, err := c.model.Stream(ctx, messages, opts...)
	if err != nil {
		return nil, err
	}

	// 包装 reader，流结束后自动将助手回复写入记忆
	return &autoRecordStreamReader{reader: reader, conv: c}, nil
}

// History 返回当前会话历史消息（不含系统提示）.
func (c *Conversation) History() []llm.Message {
	return c.memory.Messages()
}

// Reset 清空对话历史.
func (c *Conversation) Reset() {
	c.memory.Clear()
}

// buildMessages 构建发送给模型的消息列表（系统提示 + 历史）.
func (c *Conversation) buildMessages() []llm.Message {
	history := c.memory.Messages()
	if c.systemPrompt == "" {
		return history
	}
	messages := make([]llm.Message, 0, len(history)+1)
	messages = append(messages, llm.SystemMessage(c.systemPrompt))
	messages = append(messages, history...)
	return messages
}

// autoRecordStreamReader 流结束后自动将助手消息写入记忆的 StreamReader.
type autoRecordStreamReader struct {
	reader   llm.StreamReader
	conv     *Conversation
	recorded bool
}

// Recv 读取下一个片段，流结束时记录助手消息.
func (r *autoRecordStreamReader) Recv() (llm.StreamChunk, error) {
	chunk, err := r.reader.Recv()
	if err != nil && !r.recorded {
		r.recorded = true
		if resp := r.reader.Response(); resp != nil {
			r.conv.memory.Add(resp.Message)
		}
	}
	return chunk, err
}

// Response 获取完整响应.
func (r *autoRecordStreamReader) Response() *llm.ChatResponse {
	return r.reader.Response()
}

// Close 关闭流.
func (r *autoRecordStreamReader) Close() error {
	return r.reader.Close()
}

// 编译期接口断言.
var _ llm.StreamReader = (*autoRecordStreamReader)(nil)
