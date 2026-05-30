// Package locale 提供语言和地区偏好解析功能.
//
// 特性：
//   - 解析 Accept-Language 头
//   - 支持语言标签和质量值
//   - HTTP/gRPC 中间件支持
//   - 将解析结果存入 context 供链路使用
//
// 示例：
//
//	handler = locale.HTTPMiddleware()(handler)
//
//	loc := locale.FromContext(ctx)
//	fmt.Println(loc.Language())  // "zh"
//	fmt.Println(loc.Region())    // "CN"
//	fmt.Println(loc.String())    // "zh-CN"
package locale

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"strings"
)

// contextKey context 键类型.
type contextKey string

const (
	localeContextKey contextKey = "locale:locale"
)

// Locale 语言地区信息.
type Locale struct {
	// Raw 原始 Accept-Language 字符串
	Raw string

	// Preferred 首选语言标签列表按优先级排序
	Preferred []Tag
}

// Tag 语言标签.
type Tag struct {
	Language string  // 语言代码 (如 "zh", "en")
	Region   string  // 地区代码 (如 "CN", "US")
	Script   string  // 文字代码 (如 "Hans", "Hant")
	Quality  float64 // 质量值 (0.0 - 1.0)
	Raw      string  // 原始标签字符串
}

// String 返回完整的语言标签字符串.
func (t Tag) String() string {
	if t.Raw != "" {
		return t.Raw
	}
	parts := []string{t.Language}
	if t.Script != "" {
		parts = append(parts, t.Script)
	}
	if t.Region != "" {
		parts = append(parts, t.Region)
	}
	return strings.Join(parts, "-")
}

// Language 返回首选语言代码.
func (l *Locale) Language() string {
	if l == nil || len(l.Preferred) == 0 {
		return ""
	}
	return l.Preferred[0].Language
}

// Region 返回首选地区代码.
func (l *Locale) Region() string {
	if l == nil || len(l.Preferred) == 0 {
		return ""
	}
	return l.Preferred[0].Region
}

// String 返回首选语言标签字符串.
func (l *Locale) String() string {
	if l == nil || len(l.Preferred) == 0 {
		return ""
	}
	return l.Preferred[0].String()
}

// Match 检查是否匹配指定的语言.
func (l *Locale) Match(languages ...string) bool {
	if l == nil || len(l.Preferred) == 0 {
		return false
	}
	for _, tag := range l.Preferred {
		for _, lang := range languages {
			if strings.EqualFold(tag.Language, lang) {
				return true
			}
		}
	}
	return false
}

// Best 从候选列表中选择最佳匹配的语言.
func (l *Locale) Best(candidates ...string) string {
	if l == nil || len(l.Preferred) == 0 || len(candidates) == 0 {
		if len(candidates) > 0 {
			return candidates[0]
		}
		return ""
	}

	candidateMap := make(map[string]string)
	for _, c := range candidates {
		parts := strings.SplitN(c, "-", 2)
		candidateMap[strings.ToLower(parts[0])] = c
	}

	// 按优先级匹配
	for _, tag := range l.Preferred {
		lang := strings.ToLower(tag.Language)
		if match, ok := candidateMap[lang]; ok {
			return match
		}
	}

	// 没有匹配，返回第一个候选
	return candidates[0]
}

// WithLocale 将 Locale 存入 context.
func WithLocale(ctx context.Context, loc *Locale) context.Context {
	return context.WithValue(ctx, localeContextKey, loc)
}

// FromContext 从 context 获取 Locale.
func FromContext(ctx context.Context) (*Locale, bool) {
	loc, ok := ctx.Value(localeContextKey).(*Locale)
	return loc, ok
}

// GetLanguage 从 context 获取首选语言代码.
func GetLanguage(ctx context.Context) string {
	if loc, ok := FromContext(ctx); ok {
		return loc.Language()
	}
	return ""
}

// GetRegion 从 context 获取首选地区代码.
func GetRegion(ctx context.Context) string {
	if loc, ok := FromContext(ctx); ok {
		return loc.Region()
	}
	return ""
}

// GetLocale 从 context 获取首选语言标签字符串.
func GetLocale(ctx context.Context) string {
	if loc, ok := FromContext(ctx); ok {
		return loc.String()
	}
	return ""
}

// Parse 解析 Accept-Language 字符串.
func Parse(raw string) *Locale {
	loc := &Locale{Raw: raw}
	if raw == "" {
		return loc
	}

	// 解析语言标签列表
	parts := strings.Split(raw, ",")
	tags := make([]Tag, 0, len(parts))

	for _, part := range parts {
		tag := parseTag(strings.TrimSpace(part))
		if tag.Language != "" {
			tags = append(tags, tag)
		}
	}

	// 按质量值排序从高到低
	slices.SortFunc(tags, func(a, b Tag) int {
		return cmp.Compare(b.Quality, a.Quality)
	})

	loc.Preferred = tags
	return loc
}

// parseTag 解析单个语言标签.
func parseTag(s string) Tag {
	tag := Tag{Quality: 1.0}

	// 分离质量值
	parts := strings.SplitN(s, ";", 2)
	tagPart := strings.TrimSpace(parts[0])
	tag.Raw = tagPart

	if len(parts) > 1 {
		qPart := strings.TrimSpace(parts[1])
		if strings.HasPrefix(qPart, "q=") {
			if q, err := strconv.ParseFloat(qPart[2:], 64); err == nil {
				tag.Quality = q
			}
		}
	}

	// 解析语言标签各部分
	tagParts := strings.Split(tagPart, "-")
	if len(tagParts) == 0 {
		return tag
	}

	// 语言代码2-3 字母
	tag.Language = strings.ToLower(tagParts[0])

	for i := 1; i < len(tagParts); i++ {
		part := tagParts[i]
		switch {
		case len(part) == 4 && isAlpha(part):
			// 文字代码 (4 字母，首字母大写)
			tag.Script = strings.Title(strings.ToLower(part))
		case len(part) == 2 && isAlpha(part):
			// 地区代码 (2 字母，大写)
			tag.Region = strings.ToUpper(part)
		case len(part) == 3 && isDigit(part):
			// UN M.49 地区代码 (3 数字)
			tag.Region = part
		}
	}

	return tag
}

// isAlpha 检查字符串是否全是字母.
func isAlpha(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

// isDigit 检查字符串是否全是数字.
func isDigit(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
