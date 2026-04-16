package agent

import (
	"context"

	"github.com/Tsukikage7/servex/v2/llm"
)

// AgentCallbackHandler Agent 执行回调处理器.
type AgentCallbackHandler interface {
	OnAgentStart(ctx context.Context, input string)
	OnAgentEnd(ctx context.Context, result *Result, err error)
	OnLLMCall(ctx context.Context, messages []llm.Message)
	OnLLMResponse(ctx context.Context, resp *llm.ChatResponse)
	OnToolCallStart(ctx context.Context, call llm.ToolCall)
	OnToolCallEnd(ctx context.Context, call llm.ToolCall, output string, err error)
}

// NoopAgentCallbackHandler 空实现（零开销默认值）.
type NoopAgentCallbackHandler struct{}

func (NoopAgentCallbackHandler) OnAgentStart(_ context.Context, _ string)                           {}
func (NoopAgentCallbackHandler) OnAgentEnd(_ context.Context, _ *Result, _ error)                   {}
func (NoopAgentCallbackHandler) OnLLMCall(_ context.Context, _ []llm.Message)                       {}
func (NoopAgentCallbackHandler) OnLLMResponse(_ context.Context, _ *llm.ChatResponse)               {}
func (NoopAgentCallbackHandler) OnToolCallStart(_ context.Context, _ llm.ToolCall)                  {}
func (NoopAgentCallbackHandler) OnToolCallEnd(_ context.Context, _ llm.ToolCall, _ string, _ error) {}

var _ AgentCallbackHandler = NoopAgentCallbackHandler{}

// multiAgentCallbackHandler 多个 handler 合并.
type multiAgentCallbackHandler struct {
	handlers []AgentCallbackHandler
}

func (m *multiAgentCallbackHandler) OnAgentStart(ctx context.Context, input string) {
	for _, h := range m.handlers {
		func() {
			defer func() { recover() }() //nolint:errcheck
			h.OnAgentStart(ctx, input)
		}()
	}
}

func (m *multiAgentCallbackHandler) OnAgentEnd(ctx context.Context, result *Result, err error) {
	for _, h := range m.handlers {
		func() {
			defer func() { recover() }() //nolint:errcheck
			h.OnAgentEnd(ctx, result, err)
		}()
	}
}

func (m *multiAgentCallbackHandler) OnLLMCall(ctx context.Context, messages []llm.Message) {
	for _, h := range m.handlers {
		func() {
			defer func() { recover() }() //nolint:errcheck
			h.OnLLMCall(ctx, messages)
		}()
	}
}

func (m *multiAgentCallbackHandler) OnLLMResponse(ctx context.Context, resp *llm.ChatResponse) {
	for _, h := range m.handlers {
		func() {
			defer func() { recover() }() //nolint:errcheck
			h.OnLLMResponse(ctx, resp)
		}()
	}
}

func (m *multiAgentCallbackHandler) OnToolCallStart(ctx context.Context, call llm.ToolCall) {
	for _, h := range m.handlers {
		func() {
			defer func() { recover() }() //nolint:errcheck
			h.OnToolCallStart(ctx, call)
		}()
	}
}

func (m *multiAgentCallbackHandler) OnToolCallEnd(ctx context.Context, call llm.ToolCall, output string, err error) {
	for _, h := range m.handlers {
		func() {
			defer func() { recover() }() //nolint:errcheck
			h.OnToolCallEnd(ctx, call, output, err)
		}()
	}
}

// buildAgentCallbackHandler 将 callbacks 切片合并为单个 AgentCallbackHandler.
func buildAgentCallbackHandler(callbacks []AgentCallbackHandler) AgentCallbackHandler {
	switch len(callbacks) {
	case 0:
		return NoopAgentCallbackHandler{}
	case 1:
		return callbacks[0]
	default:
		return &multiAgentCallbackHandler{handlers: callbacks}
	}
}

// agentCallbacksKey context key for agent callbacks.
type agentCallbacksKey struct{}

func withAgentCallbacks(ctx context.Context, cbs []AgentCallbackHandler) context.Context {
	return context.WithValue(ctx, agentCallbacksKey{}, cbs)
}

func getAgentCallbacks(ctx context.Context) []AgentCallbackHandler {
	v, _ := ctx.Value(agentCallbacksKey{}).([]AgentCallbackHandler)
	return v
}
