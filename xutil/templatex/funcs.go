package templatex

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// builtinFuncMap 返回内置模板函数集合.
func builtinFuncMap() template.FuncMap {
	return template.FuncMap{
		"upper":    fnUpper,
		"lower":    fnLower,
		"title":    fnTitle,
		"trim":     fnTrim,
		"contains": fnContains,
		"replace":  fnReplace,
		"join":     fnJoin,
		"split":    fnSplit,
		"default":  fnDefault,
		"date":     fnDate,
		"toJSON":   fnToJSON,
		"fromJSON": fnFromJSON,
		"indent":   fnIndent,
		"plural":   fnPlural,
	}
}

// fnUpper 将字符串转为大写.
func fnUpper(s string) string {
	return strings.ToUpper(s)
}

// fnLower 将字符串转为小写.
func fnLower(s string) string {
	return strings.ToLower(s)
}

// fnTitle 将字符串转为标题格式.
func fnTitle(s string) string {
	return cases.Title(language.Und).String(s)
}

// fnTrim 去除字符串首尾空白.
func fnTrim(s string) string {
	return strings.TrimSpace(s)
}

// fnContains 判断字符串是否包含子串.
func fnContains(substr, s string) bool {
	return strings.Contains(s, substr)
}

// fnReplace 替换字符串中的子串.
func fnReplace(old, new, s string) string {
	return strings.ReplaceAll(s, old, new)
}

// fnJoin 使用分隔符连接字符串切片.
func fnJoin(sep string, elems []string) string {
	return strings.Join(elems, sep)
}

// fnSplit 使用分隔符拆分字符串.
func fnSplit(sep, s string) []string {
	return strings.Split(s, sep)
}

// fnDefault 如果值为零值则返回默认值.
func fnDefault(defaultVal, val any) any {
	if isZero(val) {
		return defaultVal
	}
	return val
}

// fnDate 格式化时间.
func fnDate(layout string, t time.Time) string {
	return t.Format(layout)
}

// fnToJSON 将值序列化为 JSON 字符串.
func fnToJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("toJSON: %w", err)
	}
	return string(b), nil
}

// fnFromJSON 将 JSON 字符串反序列化为 map.
func fnFromJSON(s string) (any, error) {
	var result any
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return nil, fmt.Errorf("fromJSON: %w", err)
	}
	return result, nil
}

// fnIndent 为多行文本添加缩进.
func fnIndent(spaces int, s string) string {
	pad := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = pad + line
		}
	}
	return strings.Join(lines, "\n")
}

// fnPlural 根据数量返回单数或复数形式.
func fnPlural(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

// isZero 检查值是否为零值.
func isZero(v any) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case bool:
		return !val
	case int:
		return val == 0
	case int64:
		return val == 0
	case float64:
		return val == 0
	default:
		return false
	}
}
