package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Tsukikage7/servex/v2/llm/mcp"
)

func TestRegistryEnforcesPolicyAndExportsLLMTools(t *testing.T) {
	registry := mcp.NewRegistry(mcp.Policy{Allow: []string{"order.lookup"}})
	err := registry.Register(mcp.Tool{
		Name:        "order.lookup",
		Description: "查询订单",
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Handler: func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
			return args, nil
		},
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := registry.Register(mcp.Tool{Name: "admin.delete"}); err == nil {
		t.Fatal("expected denied tool registration to fail")
	}

	tools := registry.LLMTools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 llm tool, got %d", len(tools))
	}
	if tools[0].Function.Name != "order.lookup" {
		t.Fatalf("unexpected tool name: %s", tools[0].Function.Name)
	}
}
