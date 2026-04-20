// Package rewrite 提供 RAG 检索前的查询改写：
//   - 历史感知改写（HistoryAwareRewriter）：基于对话历史把含代词/省略的问题改写为独立完整句子
//   - HyDE（HyDERewriter）：让 LLM 生成假设性答案用作检索查询，提升语义召回精度
//
// 常见场景：ai-support 多轮客服对话中用户常用代词或省略（"那它呢"、"这个多少钱"），
// 直接拿原始 query 做向量检索会召回错文档；检索前先调 LLM 把 query 改写为独立完整句，
// 或用 HyDE 生成假设性答案提升语义匹配。
package rewrite

import (
	"context"
	"errors"

	"github.com/Tsukikage7/servex/v2/llm"
)

// ErrNilModel 构造 Rewriter 时传入的 llm.ChatModel 为 nil.
var ErrNilModel = errors.New("rewrite: nil chat model")

// Rewriter 查询改写器接口.
type Rewriter interface {
	// Rewrite 把用户的原始 query 改写为用于检索的 query.
	// history 可能为 nil（首轮对话）.
	// 实现约定:
	//   - LLM 调用失败时返回原 query + error（调用方可选择回退到原 query）
	//   - 不含需要改写的线索时（如首轮或问题已完整）返回原 query + nil error
	Rewrite(ctx context.Context, query string, history []llm.Message) (string, error)
}

// Option 配置选项.
type Option func(*options)

// options 改写器内部配置，两个实现共用.
type options struct {
	// systemPrompt 自定义 system prompt；空则用默认模板.
	systemPrompt string
	// maxTokens 改写输出的最大 token 数.
	maxTokens int
}

// defaultMaxTokens 默认最大输出 token，适用于 HistoryAware 改写.
const defaultMaxTokens = 200

// defaultOptions 返回默认配置.
func defaultOptions() *options {
	return &options{maxTokens: defaultMaxTokens}
}

// WithSystemPrompt 自定义 system prompt.
// 支持 {{.Query}}、{{.History}} 作为模板占位符（以字符串替换方式展开，不引入 text/template 依赖）.
func WithSystemPrompt(p string) Option {
	return func(o *options) { o.systemPrompt = p }
}

// WithMaxTokens 设置改写输出的最大 token 数.
func WithMaxTokens(n int) Option {
	return func(o *options) { o.maxTokens = n }
}
