package toolcall

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm"
)

// HandlerFunc 工具处理函数.
// arguments 为 JSON 格式的参数字符串，返回 JSON 格式的结果字符串.
type HandlerFunc func(ctx context.Context, arguments string) (string, error)

// entry 注册表条目.
type entry struct {
	tool    llm.Tool
	handler HandlerFunc
}

// Registry 工具注册表，管理可用工具及其处理函数.
type Registry struct {
	entries map[string]entry
}

// NewRegistry 创建工具注册表.
func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]entry)}
}

// Register 注册工具及其处理函数.
func (r *Registry) Register(tool llm.Tool, handler HandlerFunc) {
	r.entries[tool.Function.Name] = entry{tool: tool, handler: handler}
}

// Tools 返回所有已注册工具的定义列表.
func (r *Registry) Tools() []llm.Tool {
	tools := make([]llm.Tool, 0, len(r.entries))
	for _, e := range r.entries {
		tools = append(tools, e.tool)
	}
	return tools
}

// Execute 执行指定名称的工具调用.
// 返回 JSON 格式的执行结果.
func (r *Registry) Execute(ctx context.Context, callID, name, arguments string) (string, error) {
	e, ok := r.entries[name]
	if !ok {
		return "", fmt.Errorf("toolcall: 未注册的工具: %s", name)
	}
	return e.handler(ctx, arguments)
}
