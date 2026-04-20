package router

import (
	"context"
	"errors"

	"github.com/Tsukikage7/servex/v2/llm"
)

// 编译期接口断言.
var _ llm.ChatModel = (*FallbackRouter)(nil)

// FallbackRouter 故障转移路由器.
//
// 按 models 顺序调用:主(index 0)失败且 shouldFallback 判定可降级时,依次尝试备用模型;
// 直到成功或所有模型都失败(返回最后一个错误).
//
// onFallback 在发生一次降级(from→to)时被调用,可用于监控/告警.
type FallbackRouter struct {
	models         []llm.ChatModel
	shouldFallback func(err error) bool
	onFallback     func(from, to int, err error)
}

// FallbackOption 构造选项.
type FallbackOption func(*FallbackRouter)

// WithShouldFallback 自定义"是否应当降级到下一个模型"的判定.
// 默认策略见 defaultShouldFallback.
func WithShouldFallback(fn func(error) bool) FallbackOption {
	return func(r *FallbackRouter) {
		if fn != nil {
			r.shouldFallback = fn
		}
	}
}

// WithOnFallback 设置降级 hook.
func WithOnFallback(fn func(from, to int, err error)) FallbackOption {
	return func(r *FallbackRouter) {
		r.onFallback = fn
	}
}

// defaultShouldFallback 默认降级判定(严格版,对齐 plan §B.5):
//   - context.Canceled / context.DeadlineExceeded:不降级(调用方主动取消/超时意愿明确).
//   - 其余情况,仅当 llm.IsRetryable(err)(含 429/5xx/限流)或 errors.Is(err, llm.ErrProviderUnavailable) 为真时降级.
//   - 其他业务错误(如 4xx 鉴权/请求格式错误)不降级:无意义的连环调用只会放大成本与延迟.
func defaultShouldFallback(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return llm.IsRetryable(err) || errors.Is(err, llm.ErrProviderUnavailable)
}

// NewFallbackRouter 创建故障转移路由器.
// models 顺序为主、备 1、备 2 ...
// models 为空时返回的路由器所有调用都会返回 ErrNoModels.
func NewFallbackRouter(models []llm.ChatModel, opts ...FallbackOption) *FallbackRouter {
	r := &FallbackRouter{
		models:         models,
		shouldFallback: defaultShouldFallback,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// ErrNoModels 未配置任何模型时返回.
var ErrNoModels = errors.New("router: no models configured")

// Generate 按序尝试 models;主失败且 shouldFallback 允许时降级.
func (r *FallbackRouter) Generate(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	if len(r.models) == 0 {
		return nil, ErrNoModels
	}
	var lastErr error
	for i, m := range r.models {
		resp, err := m.Generate(ctx, messages, opts...)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		// 决定是否继续尝试下一个模型.
		if i == len(r.models)-1 || !r.shouldFallback(err) {
			return nil, err
		}
		if r.onFallback != nil {
			r.onFallback(i, i+1, err)
		}
	}
	return nil, lastErr
}

// Stream 按序尝试 models 的 Stream.
// 注:一旦某个模型返回 StreamReader(err==nil),即认为成功;后续流内错误由上层处理,不再降级.
func (r *FallbackRouter) Stream(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
	if len(r.models) == 0 {
		return nil, ErrNoModels
	}
	var lastErr error
	for i, m := range r.models {
		stream, err := m.Stream(ctx, messages, opts...)
		if err == nil {
			return stream, nil
		}
		lastErr = err
		if i == len(r.models)-1 || !r.shouldFallback(err) {
			return nil, err
		}
		if r.onFallback != nil {
			r.onFallback(i, i+1, err)
		}
	}
	return nil, lastErr
}
