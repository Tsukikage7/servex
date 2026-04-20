package rewrite

import (
	"context"
	"fmt"
	"strings"

	"github.com/Tsukikage7/servex/v2/llm"
)

// defaultHyDEPrompt 默认的 HyDE 提示词模板：让 LLM 生成一段简短的假设性答案.
const defaultHyDEPrompt = `请为以下问题生成一段简短的假设性答案（2-3 句话即可，不需要真实准确，用于向量检索）。
只输出答案文本，不要添加解释或引号。

问题：{{.Query}}

假设答案：`

// hydeMaxTokens HyDE 默认最大输出 token；假设性答案一般比改写后的 query 长.
const hydeMaxTokens = 300

// hydeRewriter HyDE（Hypothetical Document Embeddings）改写器.
type hydeRewriter struct {
	// model 用于调用 LLM 生成假设性答案.
	model llm.ChatModel
	// opts 改写器配置.
	opts *options
}

// NewHyDERewriter 生成假设性答案作为检索查询（HyDE）:
//   - model 为 nil 时返回 ErrNilModel
//   - query 为空时返回原值（不触发 LLM 调用）
//   - LLM 返回空字符串（或仅空白）时返回原 query
//   - LLM 调用失败时返回原 query + error
//   - history 参数当前不使用，保留以符合 Rewriter 接口；调用方传 nil 即可
//
// HyDE 默认 maxTokens=300；如需自定义，传 WithMaxTokens 覆盖.
func NewHyDERewriter(model llm.ChatModel, opts ...Option) (Rewriter, error) {
	if model == nil {
		return nil, ErrNilModel
	}
	o := defaultOptions()
	o.maxTokens = hydeMaxTokens // HyDE 输出稍长.
	for _, opt := range opts {
		opt(o)
	}
	return &hydeRewriter{model: model, opts: o}, nil
}

// Rewrite 把 query 改写为 LLM 生成的假设性答案用于检索.
func (r *hydeRewriter) Rewrite(ctx context.Context, query string, _ []llm.Message) (string, error) {
	if query == "" {
		return query, nil
	}

	sysText := r.opts.systemPrompt
	if sysText == "" {
		sysText = defaultHyDEPrompt
	}

	// HyDE 只替换 {{.Query}}.
	rendered := strings.ReplaceAll(sysText, "{{.Query}}", query)

	resp, err := r.model.Generate(ctx, []llm.Message{llm.UserMessage(rendered)},
		llm.WithMaxTokens(r.opts.maxTokens))
	if err != nil {
		return query, fmt.Errorf("rewrite: hyde generate: %w", err)
	}
	out := strings.TrimSpace(resp.Message.Content)
	if out == "" {
		return query, nil
	}
	return out, nil
}
