// Package middleware 提供 AI 模型调用的中间件链.
package middleware

import (
	"context"

	"github.com/Tsukikage7/servex/v2/llm"
)

// Middleware AI 模型中间件，接收一个 ChatModel 返回一个装饰后的 ChatModel.
type Middleware func(llm.ChatModel) llm.ChatModel

// Chain 将多个中间件链接在一起.
// 第一个参数是最外层中间件，最后一个参数是最内层中间件最先执行.
func Chain(outer Middleware, others ...Middleware) Middleware {
	return func(next llm.ChatModel) llm.ChatModel {
		for i := len(others) - 1; i >= 0; i-- {
			next = others[i](next)
		}
		return outer(next)
	}
}

// wrappedModel 通过 generate/stream 函数对包装 ChatModel.
type wrappedModel struct {
	generateFn func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error)
	streamFn   func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error)
}

// Generate 实现 llm.ChatModel.
func (w *wrappedModel) Generate(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	return w.generateFn(ctx, messages, opts...)
}

// Stream 实现 llm.ChatModel.
func (w *wrappedModel) Stream(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
	return w.streamFn(ctx, messages, opts...)
}

// 编译期接口断言.
var _ llm.ChatModel = (*wrappedModel)(nil)

// Wrap 将 generate/stream 函数对包装为 llm.ChatModel.
func Wrap(
	generateFn func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error),
	streamFn func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error),
) llm.ChatModel {
	return &wrappedModel{
		generateFn: generateFn,
		streamFn:   streamFn,
	}
}
