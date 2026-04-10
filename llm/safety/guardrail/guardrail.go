// Package guardrail 提供 AI 输入/输出护栏，用于过滤有害内容、PII 信息及限制消息规模.
package guardrail

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/Tsukikage7/servex/llm"
	aimw "github.com/Tsukikage7/servex/llm/middleware"
)

// 护栏错误类型.
var (
	// ErrBlocked 消息被护栏拦截.
	ErrBlocked = errors.New("guardrail: message blocked")
	// ErrPIIDetected 检测到个人隐私信息.
	ErrPIIDetected = errors.New("guardrail: PII detected")
	// ErrTooLong 消息内容过长.
	ErrTooLong = errors.New("guardrail: message too long")
	// ErrTooMany 消息数量过多.
	ErrTooMany = errors.New("guardrail: too many messages")
)

// allText 提取消息中的所有文本内容（Content + Parts 中 ContentTypeText 的文本）.
func allText(m llm.Message) string {
	if len(m.Parts) == 0 {
		return m.Content
	}
	var b strings.Builder
	b.WriteString(m.Content)
	for _, p := range m.Parts {
		if p.Type == llm.ContentTypeText {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

// Guard 护栏接口.
type Guard interface {
	Check(ctx context.Context, messages []llm.Message) error
}

// GuardFunc 函数形式的 Guard.
type GuardFunc func(ctx context.Context, messages []llm.Message) error

// Check 实现 Guard 接口.
func (f GuardFunc) Check(ctx context.Context, messages []llm.Message) error {
	return f(ctx, messages)
}

// MaxLength 输入长度限制（总字符数）.
// 对所有消息的 Content 和 Parts 文本字段求 utf8 rune 数之和，超过 maxChars 时返回 ErrTooLong.
func MaxLength(maxChars int) Guard {
	return GuardFunc(func(_ context.Context, messages []llm.Message) error {
		total := 0
		for _, m := range messages {
			total += utf8.RuneCountInString(allText(m))
		}
		if total > maxChars {
			return ErrTooLong
		}
		return nil
	})
}

// MaxMessages 消息数量限制.
// 消息数量超过 n 时返回 ErrTooMany.
func MaxMessages(n int) Guard {
	return GuardFunc(func(_ context.Context, messages []llm.Message) error {
		if len(messages) > n {
			return ErrTooMany
		}
		return nil
	})
}

// KeywordFilter 关键词过滤（任一消息包含关键词则拦截）.
// 大小写不敏感匹配，命中时返回 ErrBlocked.
func KeywordFilter(keywords []string) Guard {
	// 预处理关键词为小写，避免每次检查时重复转换.
	lower := make([]string, len(keywords))
	for i, kw := range keywords {
		lower[i] = strings.ToLower(kw)
	}
	return GuardFunc(func(_ context.Context, messages []llm.Message) error {
		for _, m := range messages {
			content := strings.ToLower(allText(m))
			for _, kw := range lower {
				if strings.Contains(content, kw) {
					return ErrBlocked
				}
			}
		}
		return nil
	})
}

// RegexFilter 正则过滤.
// 任一消息内容匹配任一正则时返回 ErrBlocked.
// 若正则编译失败则 panic，调用方应保证模式合法.
func RegexFilter(patterns []string) Guard {
	compiled := make([]*regexp.Regexp, len(patterns))
	for i, p := range patterns {
		compiled[i] = regexp.MustCompile(p)
	}
	return GuardFunc(func(_ context.Context, messages []llm.Message) error {
		for _, m := range messages {
			text := allText(m)
			for _, re := range compiled {
				if re.MatchString(text) {
					return ErrBlocked
				}
			}
		}
		return nil
	})
}

// PIIPattern PII 类型.
type PIIPattern string

// 内置 PII 类型常量.
const (
	// PIIEmail 电子邮箱.
	PIIEmail PIIPattern = "email"
	// PIIPhone 手机号码.
	PIIPhone PIIPattern = "phone"
	// PIIIDCard 身份证号码.
	PIIIDCard PIIPattern = "id_card"
	// PIICreditCard 信用卡号码.
	PIICreditCard PIIPattern = "credit_card"
)

// piiRegexMap 内置 PII 正则表达式映射.
var piiRegexMap = map[PIIPattern]*regexp.Regexp{
	PIIEmail:      regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
	PIIPhone:      regexp.MustCompile(`1[3-9]\d{9}`),
	PIIIDCard:     regexp.MustCompile(`[1-9]\d{5}(18|19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]`),
	PIICreditCard: regexp.MustCompile(`\d{4}[\s\-]?\d{4}[\s\-]?\d{4}[\s\-]?\d{4}`),
}

// PIIDetector PII 检测.
// 任一消息内容命中指定 PII 类型时返回 ErrPIIDetected.
// 若未传入任何类型，则检测全部内置类型.
func PIIDetector(patterns ...PIIPattern) Guard {
	// 若未指定类型，则使用全部内置类型.
	if len(patterns) == 0 {
		patterns = []PIIPattern{PIIEmail, PIIPhone, PIIIDCard, PIICreditCard}
	}
	regs := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, ok := piiRegexMap[p]; ok {
			regs = append(regs, re)
		}
	}
	return GuardFunc(func(_ context.Context, messages []llm.Message) error {
		for _, m := range messages {
			text := allText(m)
			for _, re := range regs {
				if re.MatchString(text) {
					return ErrPIIDetected
				}
			}
		}
		return nil
	})
}

// options 护栏中间件选项.
type options struct {
	inputGuards  []Guard
	outputGuards []Guard
}

// Option 选项函数.
type Option func(*options)

// WithInputGuards 设置输入护栏，在调用模型前执行检查.
func WithInputGuards(guards ...Guard) Option {
	return func(o *options) {
		o.inputGuards = append(o.inputGuards, guards...)
	}
}

// WithOutputGuards 设置输出护栏，在模型返回后对响应消息执行检查.
func WithOutputGuards(guards ...Guard) Option {
	return func(o *options) {
		o.outputGuards = append(o.outputGuards, guards...)
	}
}

// Middleware 返回护栏中间件.
// 输入护栏在 Generate 调用前执行；输出护栏在 Generate 返回后对响应消息执行.
// 任一护栏返回错误时，立即中止并透传该错误.
func Middleware(opts ...Option) aimw.Middleware {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return func(next llm.ChatModel) llm.ChatModel {
		return aimw.Wrap(
			func(ctx context.Context, messages []llm.Message, callOpts ...llm.CallOption) (*llm.ChatResponse, error) {
				// 执行输入护栏检查.
				for _, g := range o.inputGuards {
					if err := g.Check(ctx, messages); err != nil {
						return nil, err
					}
				}
				resp, err := next.Generate(ctx, messages, callOpts...)
				if err != nil {
					return nil, err
				}
				// 执行输出护栏检查，将响应消息封装为切片传入.
				if len(o.outputGuards) > 0 {
					outMessages := []llm.Message{resp.Message}
					for _, g := range o.outputGuards {
						if err := g.Check(ctx, outMessages); err != nil {
							return nil, err
						}
					}
				}
				return resp, nil
			},
			func(ctx context.Context, messages []llm.Message, callOpts ...llm.CallOption) (llm.StreamReader, error) {
				// 流式调用同样执行输入护栏检查；输出护栏不适用于流式场景.
				for _, g := range o.inputGuards {
					if err := g.Check(ctx, messages); err != nil {
						return nil, err
					}
				}
				return next.Stream(ctx, messages, callOpts...)
			},
		)
	}
}
