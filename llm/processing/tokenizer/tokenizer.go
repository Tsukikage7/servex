// Package tokenizer 提供 LLM Token 计数与截断工具，用于成本控制和上下文窗口管理.
package tokenizer

import (
	"math"
	"unicode"

	"github.com/Tsukikage7/servex/v2/llm"
)

// Tokenizer Token 计数器接口.
type Tokenizer interface {
	// Count 估算文本的 Token 数量.
	Count(text string) int
	// CountMessages 估算消息列表的总 Token 数量（含每条消息的固定开销）.
	CountMessages(messages []llm.Message) int
	// Truncate 将文本截断至不超过 maxTokens 个 Token.
	Truncate(text string, maxTokens int) string
}

// Option 配置项函数类型.
type Option func(*options)

// options 内部配置结构.
type options struct {
	charsPerToken      float64 // ASCII 字符每 Token 占用的字符数，默认 4.0
	cjkCharsPerToken   float64 // CJK 字符每 Token 占用的字符数，默认 1.5
	overheadPerMessage int     // 每条消息的固定 Token 开销，默认 4
}

// defaultOptions 返回默认配置.
func defaultOptions() options {
	return options{
		charsPerToken:      4.0,
		cjkCharsPerToken:   1.5,
		overheadPerMessage: 4,
	}
}

// applyOptions 将 Option 列表应用到默认配置上.
func applyOptions(opts []Option) options {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// WithCharsPerToken 设置 ASCII 字符每 Token 占用的字符数（默认 4.0）.
func WithCharsPerToken(n float64) Option {
	return func(o *options) { o.charsPerToken = n }
}

// WithCJKCharsPerToken 设置 CJK 字符每 Token 占用的字符数（默认 1.5）.
func WithCJKCharsPerToken(n float64) Option {
	return func(o *options) { o.cjkCharsPerToken = n }
}

// WithOverheadPerMessage 设置每条消息的固定 Token 开销（默认 4）.
func WithOverheadPerMessage(n int) Option {
	return func(o *options) { o.overheadPerMessage = n }
}

// ──────────────────────────────────────────
// estimateTokenizer
// ──────────────────────────────────────────

type estimateTokenizer struct {
	opts options
}

// NewEstimateTokenizer 创建基于字符比例的估算 Tokenizer.
func NewEstimateTokenizer(opts ...Option) Tokenizer {
	return &estimateTokenizer{opts: applyOptions(opts)}
}

// Count 估算文本的 Token 数量.
// 遍历每个 rune，将字符分类为 CJK 或 ASCII，分别按对应比例累加 Token 成本，最终向上取整.
func (t *estimateTokenizer) Count(text string) int {
	if text == "" {
		return 0
	}
	// 以定点数（*1000）避免浮点累积误差
	var costX1000 int
	cjkCostX1000 := int(math.Round(1000.0 / t.opts.cjkCharsPerToken))
	asciiCostX1000 := int(math.Round(1000.0 / t.opts.charsPerToken))

	for _, r := range text {
		if isCJK(r) {
			costX1000 += cjkCostX1000
		} else {
			costX1000 += asciiCostX1000
		}
	}
	// 向上取整：ceil(costX1000 / 1000)
	return (costX1000 + 999) / 1000
}

// CountMessages 估算消息列表的总 Token 数量.
// 对每条消息计算其内容的 Token 数，并加上每条消息固定的开销.
func (t *estimateTokenizer) CountMessages(messages []llm.Message) int {
	total := 0
	for _, msg := range messages {
		total += t.Count(msg.Content) + t.opts.overheadPerMessage
	}
	return total
}

// Truncate 将文本截断至不超过 maxTokens 个 Token.
// 按 rune 逐字符累计 Token 成本，超出限制时停止并返回已收集的子字符串.
func (t *estimateTokenizer) Truncate(text string, maxTokens int) string {
	if text == "" || maxTokens <= 0 {
		return ""
	}
	cjkCostX1000 := int(math.Round(1000.0 / t.opts.cjkCharsPerToken))
	asciiCostX1000 := int(math.Round(1000.0 / t.opts.charsPerToken))
	limitX1000 := maxTokens * 1000

	costX1000 := 0
	var lastIdx int
	for i, r := range text {
		var c int
		if isCJK(r) {
			c = cjkCostX1000
		} else {
			c = asciiCostX1000
		}
		if costX1000+c > limitX1000 {
			return text[:i]
		}
		costX1000 += c
		lastIdx = i + len(string(r))
	}
	return text[:lastIdx]
}

// ──────────────────────────────────────────
// CL100K Tokenizer
// ──────────────────────────────────────────

// NewCL100KTokenizer 创建 CL100K 规则估算 Tokenizer（GPT-4/Claude 系列）.
// 基于 CL100K 的统计规律：英文约 4 chars/token，中文约 1.5 chars/token，每条消息固定开销 4 tokens.
func NewCL100KTokenizer() Tokenizer {
	return NewEstimateTokenizer(
		WithCharsPerToken(4.0),
		WithCJKCharsPerToken(1.5),
		WithOverheadPerMessage(4),
	)
}

// ──────────────────────────────────────────
// isCJK 判断字符是否属于中日韩统一表意文字区块
// ──────────────────────────────────────────

// isCJK 判断 rune 是否属于中日韩字符（汉字、平假名、片假名、谚文）.
func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		(r >= 0x3040 && r <= 0x309F) || // 平假名
		(r >= 0x30A0 && r <= 0x30FF) || // 片假名
		(r >= 0xAC00 && r <= 0xD7AF) // 谚文音节
}

// ──────────────────────────────────────────
// 包级辅助函数（使用默认 CL100K Tokenizer）
// ──────────────────────────────────────────

// defaultTokenizer 包级默认 CL100K Tokenizer 实例.
var defaultTokenizer = NewCL100KTokenizer()

// EstimateTokens 使用默认 CL100K Tokenizer 估算文本的 Token 数量.
func EstimateTokens(text string) int {
	return defaultTokenizer.Count(text)
}

// EstimateMessageTokens 使用默认 CL100K Tokenizer 估算消息列表的总 Token 数量.
func EstimateMessageTokens(messages []llm.Message) int {
	return defaultTokenizer.CountMessages(messages)
}

// FitsContext 判断消息列表的总 Token 数是否不超过 maxTokens.
func FitsContext(messages []llm.Message, maxTokens int) bool {
	return EstimateMessageTokens(messages) <= maxTokens
}

// TruncateToFit 截断消息列表使其总 Token 数不超过 maxTokens.
// 策略：保留所有系统消息，从最早的非系统消息开始丢弃，直至总量满足限制.
func TruncateToFit(messages []llm.Message, maxTokens int) []llm.Message {
	if FitsContext(messages, maxTokens) {
		return messages
	}

	// 分离系统消息与非系统消息，记录原始顺序中非系统消息的索引
	type indexedMsg struct {
		idx int
		msg llm.Message
	}
	var nonSystem []indexedMsg
	for i, msg := range messages {
		if msg.Role != llm.RoleSystem {
			nonSystem = append(nonSystem, indexedMsg{idx: i, msg: msg})
		}
	}

	// 构建保留集合：初始保留所有消息，从最旧的非系统消息依次丢弃
	keep := make([]bool, len(messages))
	for i := range keep {
		keep[i] = true
	}

	for _, nm := range nonSystem {
		if FitsContext(buildKept(messages, keep), maxTokens) {
			break
		}
		keep[nm.idx] = false
	}

	return buildKept(messages, keep)
}

// buildKept 根据 keep 标记构建保留的消息切片.
func buildKept(messages []llm.Message, keep []bool) []llm.Message {
	result := make([]llm.Message, 0, len(messages))
	for i, msg := range messages {
		if keep[i] {
			result = append(result, msg)
		}
	}
	return result
}
