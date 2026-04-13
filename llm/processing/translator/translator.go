// Package translator 提供基于 LLM 的文本翻译功能，支持单文本翻译、批量翻译及语言检测.
package translator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Tsukikage7/servex/v2/llm"
)

// 翻译器错误类型.
var (
	// ErrNilModel 模型为 nil 时返回.
	ErrNilModel = errors.New("translator: model is nil")
	// ErrEmptyText 文本为空时返回.
	ErrEmptyText = errors.New("translator: empty text")
	// ErrEmptyTarget 目标语言为空时返回.
	ErrEmptyTarget = errors.New("translator: target language is empty")
)

// Translation 翻译结果.
type Translation struct {
	// Text 翻译后的文本.
	Text string `json:"text"`
	// SourceLanguage 源语言代码.
	SourceLanguage string `json:"source_language"`
	// TargetLanguage 目标语言代码.
	TargetLanguage string `json:"target_language"`
}

// BatchTranslation 批量翻译结果.
type BatchTranslation struct {
	// Translations 翻译结果列表，与输入文本一一对应.
	Translations []Translation `json:"translations"`
}

// Translator 翻译器接口.
type Translator interface {
	// Translate 将单段文本翻译为目标语言.
	Translate(ctx context.Context, text string, targetLang string) (*Translation, error)
	// TranslateBatch 批量翻译文本列表.
	TranslateBatch(ctx context.Context, texts []string, targetLang string) (*BatchTranslation, error)
	// DetectLanguage 检测文本的语言，返回语言代码（如 "zh"、"en"、"ja"）.
	DetectLanguage(ctx context.Context, text string) (string, error)
}

// options 翻译器内部选项.
type options struct {
	// callOpts 底层模型调用选项.
	callOpts []llm.CallOption
	// sourceLang 源语言代码，为空则自动检测.
	sourceLang string
	// glossary 术语表，key 为原文，value 为译文.
	glossary map[string]string
	// tone 翻译风格（formal/informal/technical）.
	tone string
	// batchSize 批量翻译每批大小，默认 10.
	batchSize int
}

// Option 翻译器选项函数.
type Option func(*options)

// WithCallOptions 设置底层模型调用选项.
func WithCallOptions(opts ...llm.CallOption) Option {
	return func(o *options) {
		o.callOpts = append(o.callOpts, opts...)
	}
}

// WithSourceLanguage 指定源语言（可选，默认自动检测）.
func WithSourceLanguage(lang string) Option {
	return func(o *options) {
		o.sourceLang = lang
	}
}

// WithGlossary 设置术语表，确保专业词汇的翻译一致性.
// key 为源语言词汇，value 为目标语言对应译文.
func WithGlossary(glossary map[string]string) Option {
	return func(o *options) {
		o.glossary = glossary
	}
}

// WithTone 设置翻译风格（formal 正式/informal 口语/technical 技术）.
func WithTone(tone string) Option {
	return func(o *options) {
		o.tone = tone
	}
}

// WithBatchSize 设置批量翻译每批的最大文本数（默认 10）.
func WithBatchSize(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.batchSize = n
		}
	}
}

// llmTranslator 基于 LLM 的翻译器实现.
type llmTranslator struct {
	model llm.ChatModel
	opts  *options
}

// NewTranslator 创建基于 LLM 的翻译器.
func NewTranslator(model llm.ChatModel, opts ...Option) Translator {
	o := &options{batchSize: 10}
	for _, opt := range opts {
		opt(o)
	}
	return &llmTranslator{model: model, opts: o}
}

// Translate 将单段文本翻译为目标语言.
func (t *llmTranslator) Translate(ctx context.Context, text string, targetLang string) (*Translation, error) {
	if t.model == nil {
		return nil, ErrNilModel
	}
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmptyText
	}
	if strings.TrimSpace(targetLang) == "" {
		return nil, ErrEmptyTarget
	}

	sysPrompt := t.buildTranslatePrompt(targetLang)

	messages := []llm.Message{
		llm.SystemMessage(sysPrompt),
		llm.UserMessage(text),
	}

	resp, err := t.model.Generate(ctx, messages, t.opts.callOpts...)
	if err != nil {
		return nil, fmt.Errorf("translator: 模型调用失败: %w", err)
	}

	content := llm.ExtractJSON(resp.Message.Content)
	var result Translation
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("translator: JSON 解析失败: %w", err)
	}

	return &result, nil
}

// buildTranslatePrompt 构造翻译系统提示.
func (t *llmTranslator) buildTranslatePrompt(targetLang string) string {
	var sb strings.Builder

	sb.WriteString("你是专业翻译。将以下文本翻译为")
	sb.WriteString(targetLang)
	sb.WriteString("。")

	// 源语言提示.
	if t.opts.sourceLang != "" {
		sb.WriteString(fmt.Sprintf("源语言为 %s。", t.opts.sourceLang))
	}

	// 术语表.
	if len(t.opts.glossary) > 0 {
		sb.WriteString("术语表（请严格遵循）：")
		for k, v := range t.opts.glossary {
			sb.WriteString(fmt.Sprintf("%s→%s；", k, v))
		}
	}

	// 翻译风格.
	if t.opts.tone != "" {
		toneDesc := map[string]string{
			"formal":    "正式",
			"informal":  "口语化",
			"technical": "技术性",
		}
		if desc, ok := toneDesc[t.opts.tone]; ok {
			sb.WriteString(fmt.Sprintf("翻译风格：%s。", desc))
		} else {
			sb.WriteString(fmt.Sprintf("翻译风格：%s。", t.opts.tone))
		}
	}

	sb.WriteString(`输出 JSON：{"text": "翻译结果", "source_language": "源语言代码", "target_language": "目标语言代码"}。仅输出 JSON，不要其他内容。`)

	return sb.String()
}

// TranslateBatch 批量翻译文本列表，按 batchSize 分批处理.
func (t *llmTranslator) TranslateBatch(ctx context.Context, texts []string, targetLang string) (*BatchTranslation, error) {
	if t.model == nil {
		return nil, ErrNilModel
	}
	if strings.TrimSpace(targetLang) == "" {
		return nil, ErrEmptyTarget
	}

	var allTranslations []Translation

	// 按 batchSize 分批处理.
	for i := 0; i < len(texts); i += t.opts.batchSize {
		end := i + t.opts.batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		translations, err := t.translateBatch(ctx, batch, targetLang)
		if err != nil {
			return nil, err
		}
		allTranslations = append(allTranslations, translations...)
	}

	return &BatchTranslation{Translations: allTranslations}, nil
}

// translateBatch 翻译单批文本.
func (t *llmTranslator) translateBatch(ctx context.Context, texts []string, targetLang string) ([]Translation, error) {
	// 构造编号文本.
	var numbered strings.Builder
	for i, text := range texts {
		numbered.WriteString(fmt.Sprintf("%d. %s\n", i+1, text))
	}

	sysPrompt := fmt.Sprintf(
		"你是专业翻译。将以下编号文本列表翻译为 %s，保持编号不变。"+
			"输出 JSON 数组，每个元素包含：text（译文）、source_language（源语言代码）、target_language（目标语言代码）。"+
			"数组顺序与输入编号一致。仅输出 JSON 数组。",
		targetLang,
	)

	messages := []llm.Message{
		llm.SystemMessage(sysPrompt),
		llm.UserMessage(numbered.String()),
	}

	resp, err := t.model.Generate(ctx, messages, t.opts.callOpts...)
	if err != nil {
		return nil, fmt.Errorf("translator: 批量翻译失败: %w", err)
	}

	content := llm.ExtractJSON(resp.Message.Content)
	var translations []Translation
	if err := json.Unmarshal([]byte(content), &translations); err != nil {
		return nil, fmt.Errorf("translator: JSON 解析失败: %w", err)
	}

	return translations, nil
}

// DetectLanguage 检测文本语言，返回语言代码.
func (t *llmTranslator) DetectLanguage(ctx context.Context, text string) (string, error) {
	if t.model == nil {
		return "", ErrNilModel
	}
	if strings.TrimSpace(text) == "" {
		return "", ErrEmptyText
	}

	sysPrompt := "检测以下文本的语言，输出语言代码（如 zh、en、ja、ko、fr、de、es 等）。" +
		"仅输出一个语言代码字符串，不要任何额外内容，不要引号或标点。"

	messages := []llm.Message{
		llm.SystemMessage(sysPrompt),
		llm.UserMessage(text),
	}

	resp, err := t.model.Generate(ctx, messages, t.opts.callOpts...)
	if err != nil {
		return "", fmt.Errorf("translator: 语言检测失败: %w", err)
	}

	lang := strings.TrimSpace(resp.Message.Content)
	// 去除可能的引号.
	lang = strings.Trim(lang, `"'`)
	return lang, nil
}
