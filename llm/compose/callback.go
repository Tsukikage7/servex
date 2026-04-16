package compose

import (
	"context"
)

// CallbackHandler Graph 执行回调处理器.
type CallbackHandler interface {
	OnGraphStart(ctx context.Context, info GraphRunInfo, input any)
	OnGraphEnd(ctx context.Context, info GraphRunInfo, output any, err error)
	OnNodeStart(ctx context.Context, info NodeRunInfo, input any)
	OnNodeEnd(ctx context.Context, info NodeRunInfo, output any, err error)
	OnNodeSkip(ctx context.Context, info NodeRunInfo, reason string)
}

// GraphRunInfo 图执行信息.
type GraphRunInfo struct {
	RunID string
}

// NodeRunInfo 节点执行信息.
type NodeRunInfo struct {
	NodeID   string
	NodeKind string // "invoke", "stream", "collect", "transform"
	GraphRunInfo
}

// NoopCallbackHandler 空实现（零开销默认值）.
type NoopCallbackHandler struct{}

func (NoopCallbackHandler) OnGraphStart(_ context.Context, _ GraphRunInfo, _ any)              {}
func (NoopCallbackHandler) OnGraphEnd(_ context.Context, _ GraphRunInfo, _ any, _ error)       {}
func (NoopCallbackHandler) OnNodeStart(_ context.Context, _ NodeRunInfo, _ any)                {}
func (NoopCallbackHandler) OnNodeEnd(_ context.Context, _ NodeRunInfo, _ any, _ error)         {}
func (NoopCallbackHandler) OnNodeSkip(_ context.Context, _ NodeRunInfo, _ string)              {}

var _ CallbackHandler = NoopCallbackHandler{}

// multiCallbackHandler 多个 handler 合并.
type multiCallbackHandler struct {
	handlers []CallbackHandler
}

func (m *multiCallbackHandler) OnGraphStart(ctx context.Context, info GraphRunInfo, input any) {
	for _, h := range m.handlers {
		func() {
			defer func() { recover() }() //nolint:errcheck
			h.OnGraphStart(ctx, info, input)
		}()
	}
}

func (m *multiCallbackHandler) OnGraphEnd(ctx context.Context, info GraphRunInfo, output any, err error) {
	for _, h := range m.handlers {
		func() {
			defer func() { recover() }() //nolint:errcheck
			h.OnGraphEnd(ctx, info, output, err)
		}()
	}
}

func (m *multiCallbackHandler) OnNodeStart(ctx context.Context, info NodeRunInfo, input any) {
	for _, h := range m.handlers {
		func() {
			defer func() { recover() }() //nolint:errcheck
			h.OnNodeStart(ctx, info, input)
		}()
	}
}

func (m *multiCallbackHandler) OnNodeEnd(ctx context.Context, info NodeRunInfo, output any, err error) {
	for _, h := range m.handlers {
		func() {
			defer func() { recover() }() //nolint:errcheck
			h.OnNodeEnd(ctx, info, output, err)
		}()
	}
}

func (m *multiCallbackHandler) OnNodeSkip(ctx context.Context, info NodeRunInfo, reason string) {
	for _, h := range m.handlers {
		func() {
			defer func() { recover() }() //nolint:errcheck
			h.OnNodeSkip(ctx, info, reason)
		}()
	}
}

// CompileOption Compile 时的选项.
type CompileOption func(*compileOptions)

type compileOptions struct {
	callbacks []CallbackHandler
}

// WithCallbacks 注入回调处理器.
func WithCallbacks(handlers ...CallbackHandler) CompileOption {
	return func(o *compileOptions) { o.callbacks = append(o.callbacks, handlers...) }
}

// buildCallbackHandler 将 callbacks 切片合并为单个 CallbackHandler.
// 空切片返回 NoopCallbackHandler（零开销快速路径）.
func buildCallbackHandler(callbacks []CallbackHandler) CallbackHandler {
	switch len(callbacks) {
	case 0:
		return NoopCallbackHandler{}
	case 1:
		return callbacks[0]
	default:
		return &multiCallbackHandler{handlers: callbacks}
	}
}

// nodeKindString 将 nodeKind 转为字符串.
func nodeKindString(k nodeKind) string {
	switch k {
	case kindInvoke:
		return "invoke"
	case kindStream:
		return "stream"
	case kindCollect:
		return "collect"
	case kindTransform:
		return "transform"
	default:
		return "unknown"
	}
}
