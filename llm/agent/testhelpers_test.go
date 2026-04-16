package agent_test

import (
	"context"
	"io"

	"github.com/Tsukikage7/servex/v2/llm"
)

// fixedModel 返回固定文本的 mock ChatModel.
type fixedModel struct{ reply string }

func (m *fixedModel) Generate(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Message: llm.AssistantMessage(m.reply)}, nil
}

func (m *fixedModel) Stream(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (llm.StreamReader, error) {
	return &singleTokenReader{token: m.reply}, nil
}

// singleTokenReader 单 token 流 mock.
type singleTokenReader struct {
	token string
	sent  bool
}

func (r *singleTokenReader) Recv() (llm.StreamChunk, error) {
	if r.sent {
		return llm.StreamChunk{}, io.EOF
	}
	r.sent = true
	return llm.StreamChunk{Delta: r.token}, nil
}

func (r *singleTokenReader) Response() *llm.ChatResponse {
	return &llm.ChatResponse{Message: llm.AssistantMessage(r.token)}
}

func (r *singleTokenReader) Close() error { return nil }

// errorModel 总是返回错误的 mock.
type errorModel struct{ err error }

func (m *errorModel) Generate(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
	return nil, m.err
}

func (m *errorModel) Stream(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (llm.StreamReader, error) {
	return nil, m.err
}
