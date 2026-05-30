package llm

import "context"

// StreamReader 流式读取器迭代器模式.
// 使用方式：循环调用 Recv() 直到返回 io.EOF，最后调用 Close().
type StreamReader interface {
	// Recv 读取下一个响应片段.
	// 返回 io.EOF 表示流已结束.
	Recv() (StreamChunk, error)
	// Response 流结束后获取累积的完整响应.
	// 在收到 io.EOF 之前调用返回 nil.
	Response() *ChatResponse
	// Close 关闭流并释放资源.
	Close() error
}

// StreamChunk 流式响应片段.
type StreamChunk struct {
	// Delta 本次增量文本内容.
	Delta string
	// ToolCalls 工具调用片段流式工具调用时使用.
	ToolCalls []ToolCall
	// FinishReason 停止原因最后一个 chunk 时非空.
	FinishReason string
}

// StreamCallback 流式回调函数.
// 在 Generate 中配合 WithStreamCallback 使用，边生成边回调.
// 返回非 nil error 时中断流式生成.
type StreamCallback func(ctx context.Context, chunk StreamChunk) error
