package observability_test

import (
	"testing"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/observability"
)

func TestModelAndUsageAttributes(t *testing.T) {
	attrs := append(
		observability.ModelAttributes("openai", "gpt-4o-mini"),
		observability.UsageAttributes(llm.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15})...,
	)

	got := map[string]string{}
	for _, attr := range attrs {
		got[string(attr.Key)] = attr.Value.AsString()
	}
	if got[observability.AttrSystem] != "openai" {
		t.Fatalf("unexpected system attr: %q", got[observability.AttrSystem])
	}
	if got[observability.AttrRequestModel] != "gpt-4o-mini" {
		t.Fatalf("unexpected model attr: %q", got[observability.AttrRequestModel])
	}
}
