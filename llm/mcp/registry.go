// Package mcp provides a small tool gateway boundary for Model Context Protocol
// integrations. It intentionally does not implement an agent loop.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"

	"github.com/Tsukikage7/servex/v2/llm"
)

var (
	ErrToolDenied    = errors.New("mcp: tool denied by policy")
	ErrToolExists    = errors.New("mcp: tool already registered")
	ErrToolNotFound  = errors.New("mcp: tool not found")
	ErrInvalidTool   = errors.New("mcp: invalid tool")
	ErrMissingHandle = errors.New("mcp: missing tool handler")
)

type ToolHandler func(ctx context.Context, args json.RawMessage) (json.RawMessage, error)

type Tool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
	Handler     ToolHandler
}

type Client interface {
	ListTools(ctx context.Context) ([]Tool, error)
	CallTool(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error)
}

type Policy struct {
	Allow []string
	Deny  []string
}

func (p Policy) Allows(name string) bool {
	if slices.Contains(p.Deny, name) {
		return false
	}
	return len(p.Allow) == 0 || slices.Contains(p.Allow, name)
}

type Registry struct {
	mu     sync.RWMutex
	policy Policy
	tools  map[string]Tool
}

func NewRegistry(policy Policy) *Registry {
	return &Registry{
		policy: policy,
		tools:  make(map[string]Tool),
	}
}

func (r *Registry) Register(tool Tool) error {
	if tool.Name == "" {
		return ErrInvalidTool
	}
	if !r.policy.Allows(tool.Name) {
		return ErrToolDenied
	}
	if tool.Handler == nil {
		return ErrMissingHandle
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.tools[tool.Name]; ok {
		return ErrToolExists
	}
	r.tools[tool.Name] = tool
	return nil
}

func (r *Registry) Lookup(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

func (r *Registry) Call(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	tool, ok := r.Lookup(name)
	if !ok {
		return nil, ErrToolNotFound
	}
	return tool.Handler(ctx, args)
}

func (r *Registry) Tools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	slices.SortFunc(tools, func(a, b Tool) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
	return tools
}

func (r *Registry) LLMTools() []llm.Tool {
	tools := r.Tools()
	result := make([]llm.Tool, 0, len(tools))
	for _, tool := range tools {
		result = append(result, llm.Tool{
			Function: llm.FunctionDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}
	return result
}
