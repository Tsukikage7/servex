package middleware

import (
	"context"
	"sync"

	"github.com/Tsukikage7/servex/v2/llm"
)

// UsageTracker 线程安全的 token 用量累计追踪器.
type UsageTracker struct {
	mu    sync.Mutex
	total llm.Usage
}

// Middleware 返回用量追踪中间件.
func (t *UsageTracker) Middleware() Middleware {
	return func(next llm.ChatModel) llm.ChatModel {
		return Wrap(
			func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
				resp, err := next.Generate(ctx, messages, opts...)
				if err == nil && resp != nil {
					t.mu.Lock()
					t.total.Add(resp.Usage)
					t.mu.Unlock()
				}
				return resp, err
			},
			func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
				// 流式接口：包装 StreamReader，在流结束后累计用量
				reader, err := next.Stream(ctx, messages, opts...)
				if err != nil {
					return nil, err
				}
				return &trackingStreamReader{reader: reader, tracker: t}, nil
			},
		)
	}
}

// Total 返回累计总用量线程安全.
func (t *UsageTracker) Total() llm.Usage {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.total
}

// Reset 重置累计用量线程安全.
func (t *UsageTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.total = llm.Usage{}
}

// trackingStreamReader 在流结束后记录用量的 StreamReader 包装器.
type trackingStreamReader struct {
	reader  llm.StreamReader
	tracker *UsageTracker
	done    bool
}

// Recv 读取下一个片段，流结束时记录用量.
func (r *trackingStreamReader) Recv() (llm.StreamChunk, error) {
	chunk, err := r.reader.Recv()
	if err != nil && !r.done {
		r.done = true
		if resp := r.reader.Response(); resp != nil {
			r.tracker.mu.Lock()
			r.tracker.total.Add(resp.Usage)
			r.tracker.mu.Unlock()
		}
	}
	return chunk, err
}

// Response 获取完整响应.
func (r *trackingStreamReader) Response() *llm.ChatResponse {
	return r.reader.Response()
}

// Close 关闭流.
func (r *trackingStreamReader) Close() error {
	return r.reader.Close()
}

// 编译期接口断言.
var _ llm.StreamReader = (*trackingStreamReader)(nil)
