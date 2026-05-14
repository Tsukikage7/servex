// Package llm 提供 LLM/AI 服务的统一客户端抽象.
//
// 支持多 Provider（OpenAI、Anthropic、Gemini 等），提供统一的 ChatModel 和 EmbeddingModel 接口。
// 复杂 Agent、Graph、RAG 编排由 llm/framework/eino 和 llm/framework/adk 对接开源框架承载。
package llm

import "context"

// ChatModel 聊天模型接口，所有 Provider 均实现此接口.
type ChatModel interface {
	// Generate 非流式生成，返回完整响应.
	Generate(ctx context.Context, messages []Message, opts ...CallOption) (*ChatResponse, error)
	// Stream 流式生成，返回 StreamReader 供逐块读取.
	Stream(ctx context.Context, messages []Message, opts ...CallOption) (StreamReader, error)
}

// EmbeddingModel 文本嵌入模型接口.
type EmbeddingModel interface {
	// EmbedTexts 将文本列表转换为向量表示.
	EmbedTexts(ctx context.Context, texts []string, opts ...CallOption) (*EmbedResponse, error)
}

// ChatResponse 聊天响应.
type ChatResponse struct {
	// Message 模型生成的消息.
	Message Message
	// Usage token 用量统计.
	Usage Usage
	// FinishReason 停止原因："stop", "tool_calls", "length", "content_filter".
	FinishReason string
	// ModelID 实际使用的模型 ID.
	ModelID string
}

// EmbedResponse 嵌入响应.
type EmbedResponse struct {
	// Embeddings 向量列表，与输入文本一一对应.
	Embeddings [][]float32
	// Usage token 用量统计.
	Usage Usage
	// ModelID 实际使用的模型 ID.
	ModelID string
}
