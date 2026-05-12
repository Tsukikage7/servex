// Package i18n 提供国际化本地化支持.
// 基于 golang.org/x/text/language 实现语言匹配，使用 JSON 文件存储翻译消息，
// 支持 text/template 模板语法进行参数替换.
package i18n

import (
	"bytes"
	"encoding/json"
	"os"
	"sync"
	"text/template"

	"golang.org/x/text/language"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Bundle 消息包，管理多语言消息文件.
type Bundle struct {
	defaultTag language.Tag
	matcher    language.Matcher
	logger     logger.Logger

	mu       sync.RWMutex
	messages map[language.Tag]map[string]string // tag -> messageID -> message
	tags     []language.Tag                     // 已注册的语言标签（用于构建 matcher）
}

// Option Bundle 配置选项.
type Option func(*Bundle)

// WithLogger 设置日志记录器.
func WithLogger(l logger.Logger) Option {
	return func(b *Bundle) {
		b.logger = l
	}
}

// NewBundle 创建消息包.
func NewBundle(defaultLang language.Tag, opts ...Option) *Bundle {
	b := &Bundle{
		defaultTag: defaultLang,
		messages:   make(map[language.Tag]map[string]string),
		tags:       []language.Tag{defaultLang},
	}
	for _, opt := range opts {
		opt(b)
	}
	b.matcher = language.NewMatcher(b.tags)
	return b
}

// LoadMessageFile 加载 JSON 消息文件.
// 文件名中须包含语言标签（如 messages.zh.json、en.json），
// 语言标签通过 tag 参数显式指定.
func (b *Bundle) LoadMessageFile(tag language.Tag, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if b.logger != nil {
			b.logger.With(
				logger.String("file", path),
				logger.Err(err),
			).Warn("[I18n] 消息文件加载失败")
		}
		return err
	}

	var messages map[string]string
	if err := json.Unmarshal(data, &messages); err != nil {
		if b.logger != nil {
			b.logger.With(
				logger.String("file", path),
				logger.Err(err),
			).Warn("[I18n] 消息文件解析失败")
		}
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.messages[tag]; !exists {
		b.tags = append(b.tags, tag)
	}
	b.messages[tag] = messages
	b.matcher = language.NewMatcher(b.tags)

	if b.logger != nil {
		b.logger.With(logger.String("file", path)).Debug("[I18n] 消息文件已加载")
	}
	return nil
}

// LoadMessages 直接注册消息映射.
func (b *Bundle) LoadMessages(tag language.Tag, messages map[string]string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.messages[tag]; !exists {
		b.tags = append(b.tags, tag)
	}
	cp := make(map[string]string, len(messages))
	for k, v := range messages {
		cp[k] = v
	}
	b.messages[tag] = cp
	b.matcher = language.NewMatcher(b.tags)
}

// NewLocalizer 创建本地化器.
// langs 为语言偏好列表（如 Accept-Language 头），按优先级从高到低排列.
func (b *Bundle) NewLocalizer(langs ...string) *Localizer {
	parsedTags := make([]language.Tag, 0, len(langs))
	for _, l := range langs {
		t, err := language.Parse(l)
		if err == nil {
			parsedTags = append(parsedTags, t)
		}
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	// 使用 index 查找原始注册 tag，避免规范化后 == 比较失败
	_, idx, _ := b.matcher.Match(parsedTags...)
	matchedTag := b.tags[idx]

	// 深拷贝消息映射，避免 Localizer 持有 Bundle 内部 map 的引用导致并发读写竞争.
	msgsCopy := make(map[language.Tag]map[string]string, len(b.messages))
	for tag, msgs := range b.messages {
		cp := make(map[string]string, len(msgs))
		for k, v := range msgs {
			cp[k] = v
		}
		msgsCopy[tag] = cp
	}

	return &Localizer{
		tag:        matchedTag,
		defaultTag: b.defaultTag,
		messages:   msgsCopy,
	}
}

// Localizer 本地化器，用于翻译消息.
type Localizer struct {
	tag        language.Tag
	defaultTag language.Tag
	messages   map[language.Tag]map[string]string
}

// Translate 翻译消息，失败时返回 messageID.
func (l *Localizer) Translate(messageID string, data ...map[string]any) string {
	msg := l.resolve(messageID)
	if msg == "" {
		return messageID
	}
	return l.render(msg, data)
}

// MustTranslate 翻译消息，失败时返回 defaultMsg.
func (l *Localizer) MustTranslate(messageID, defaultMsg string, data ...map[string]any) string {
	msg := l.resolve(messageID)
	if msg == "" {
		return defaultMsg
	}
	return l.render(msg, data)
}

// resolve 按优先级查找消息：匹配语言 -> 默认语言.
func (l *Localizer) resolve(messageID string) string {
	if msgs, ok := l.messages[l.tag]; ok {
		if msg, ok := msgs[messageID]; ok {
			return msg
		}
	}
	if l.tag != l.defaultTag {
		if msgs, ok := l.messages[l.defaultTag]; ok {
			if msg, ok := msgs[messageID]; ok {
				return msg
			}
		}
	}
	return ""
}

// render 使用 text/template 渲染模板数据.
func (l *Localizer) render(msg string, data []map[string]any) string {
	if len(data) == 0 || data[0] == nil {
		return msg
	}

	tmpl, err := template.New("").Parse(msg)
	if err != nil {
		return msg
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data[0]); err != nil {
		return msg
	}
	return buf.String()
}
