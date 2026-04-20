package rewrite

import (
	"context"
	"fmt"
	"strings"

	"github.com/Tsukikage7/servex/v2/llm"
)

// defaultHistoryAwarePrompt 默认的历史感知改写提示词模板.
const defaultHistoryAwarePrompt = `你是一个问题改写助手。
根据给出的对话历史，把用户的【当前问题】改写为独立、完整、不含代词或省略的查询句子。
只输出改写后的问题，不要添加解释、前缀或引号。

# 对话历史
{{.History}}

# 当前问题
{{.Query}}

改写结果：`

// maxHistoryMessages 历史消息最大保留条数（仅保留最近 N 条非 system 消息）.
const maxHistoryMessages = 10

// historyAwareRewriter 基于对话历史的查询改写器.
type historyAwareRewriter struct {
	// model 用于调用 LLM 生成改写结果.
	model llm.ChatModel
	// opts 改写器配置.
	opts *options
}

// NewHistoryAwareRewriter 基于对话历史改写代词与省略:
//   - model 为 nil 时返回 ErrNilModel
//   - history 为空或 nil 时直接返回原 query（不触发 LLM 调用）
//   - query 为空时直接返回原 query（不触发 LLM 调用）
//   - LLM 返回空字符串（或仅空白）时返回原 query（认为无需改写）
//   - LLM 调用失败时返回原 query + error
func NewHistoryAwareRewriter(model llm.ChatModel, opts ...Option) (Rewriter, error) {
	if model == nil {
		return nil, ErrNilModel
	}
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &historyAwareRewriter{model: model, opts: o}, nil
}

// Rewrite 根据对话历史把 query 改写为独立完整的检索查询.
func (r *historyAwareRewriter) Rewrite(ctx context.Context, query string, history []llm.Message) (string, error) {
	if query == "" || len(history) == 0 {
		return query, nil
	}

	sysText := r.opts.systemPrompt
	if sysText == "" {
		sysText = defaultHistoryAwarePrompt
	}

	historyStr := formatHistory(history)
	// 模板渲染：简单字符串替换 {{.History}}、{{.Query}}，不引入 text/template 依赖.
	rendered := strings.ReplaceAll(sysText, "{{.History}}", historyStr)
	rendered = strings.ReplaceAll(rendered, "{{.Query}}", query)

	resp, err := r.model.Generate(ctx, []llm.Message{llm.UserMessage(rendered)},
		llm.WithMaxTokens(r.opts.maxTokens))
	if err != nil {
		return query, fmt.Errorf("rewrite: history-aware generate: %w", err)
	}
	out := strings.TrimSpace(resp.Message.Content)
	if out == "" {
		return query, nil
	}
	return out, nil
}

// formatHistory 把 []llm.Message 拼成多行字符串："角色: 内容\n...".
// 截断策略：保留最近 maxHistoryMessages 条，跳过 system 角色.
// 角色映射中文：user → "用户"，assistant → "助手"，其它（非 system）按原值输出.
func formatHistory(history []llm.Message) string {
	// 先过滤 system 消息.
	filtered := make([]llm.Message, 0, len(history))
	for _, m := range history {
		if m.Role == llm.RoleSystem {
			continue
		}
		filtered = append(filtered, m)
	}
	// 仅保留最近 N 条.
	if len(filtered) > maxHistoryMessages {
		filtered = filtered[len(filtered)-maxHistoryMessages:]
	}

	var b strings.Builder
	for i, m := range filtered {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(roleLabel(m.Role))
		b.WriteString(": ")
		b.WriteString(m.Content)
	}
	return b.String()
}

// roleLabel 把角色映射为中文显示标签.
func roleLabel(r llm.Role) string {
	switch r {
	case llm.RoleUser:
		return "用户"
	case llm.RoleAssistant:
		return "助手"
	default:
		return string(r)
	}
}
