// Package observability provides small OpenTelemetry GenAI attribute helpers
// for servex LLM integrations.
package observability

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/Tsukikage7/servex/v2/llm"
)

const (
	AttrSystem            = "gen_ai.system"
	AttrRequestModel      = "gen_ai.request.model"
	AttrResponseModel     = "gen_ai.response.model"
	AttrUsageInputTokens  = "gen_ai.usage.input_tokens"
	AttrUsageOutputTokens = "gen_ai.usage.output_tokens"
	AttrUsageTotalTokens  = "gen_ai.usage.total_tokens"
	AttrToolName          = "gen_ai.tool.name"
	AttrCacheHit          = "gen_ai.cache.hit"
	AttrFallbackFrom      = "gen_ai.fallback.from"
	AttrFallbackTo        = "gen_ai.fallback.to"
)

func ModelAttributes(provider, model string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrSystem, provider),
		attribute.String(AttrRequestModel, model),
	}
}

func UsageAttributes(usage llm.Usage) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.Int(AttrUsageInputTokens, usage.PromptTokens),
		attribute.Int(AttrUsageOutputTokens, usage.CompletionTokens),
		attribute.Int(AttrUsageTotalTokens, usage.TotalTokens),
	}
}

func ToolCallAttributes(name string) []attribute.KeyValue {
	return []attribute.KeyValue{attribute.String(AttrToolName, name)}
}

func CacheAttributes(hit bool) []attribute.KeyValue {
	return []attribute.KeyValue{attribute.Bool(AttrCacheHit, hit)}
}

func FallbackAttributes(from, to string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrFallbackFrom, from),
		attribute.String(AttrFallbackTo, to),
	}
}

func RecordUsage(span trace.Span, usage llm.Usage) {
	if span == nil {
		return
	}
	span.SetAttributes(UsageAttributes(usage)...)
}
