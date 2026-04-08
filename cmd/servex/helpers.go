package main

import (
	"strings"
	"unicode"
)

// toPascalCase 将 snake_case 或 kebab-case 转为 PascalCase.
func toPascalCase(s string) string {
	parts := splitWords(s)
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + strings.ToLower(p[1:]))
	}
	return b.String()
}

// toCamelCase 将 snake_case 或 kebab-case 转为 camelCase.
func toCamelCase(s string) string {
	pascal := toPascalCase(s)
	if pascal == "" {
		return ""
	}
	return strings.ToLower(pascal[:1]) + pascal[1:]
}

// toSnakeCase 将 PascalCase 或 camelCase 转为 snake_case.
func toSnakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// splitWords 按下划线、连字符或大小写边界分词.
func splitWords(s string) []string {
	s = strings.ReplaceAll(s, "-", "_")
	return strings.Split(s, "_")
}

// goType 将简写类型映射为 Go 类型.
func goType(t string) string {
	switch t {
	case "string":
		return "string"
	case "int":
		return "int"
	case "int64":
		return "int64"
	case "uint":
		return "uint"
	case "uint64":
		return "uint64"
	case "float64":
		return "float64"
	case "bool":
		return "bool"
	case "time.Time":
		return "time.Time"
	default:
		return t
	}
}

// zeroValue 返回 Go 类型的零值字面量[用于测试模板].
func zeroValue(typ string) string {
	switch typ {
	case "string":
		return `""`
	case "int", "int8", "int16", "int32", "int64":
		return "0"
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return "0"
	case "float32", "float64":
		return "0"
	case "bool":
		return "false"
	case "time.Time":
		return "time.Time{}"
	default:
		return `""`
	}
}

// needsTimeImport 检查字段列表是否包含 time.Time 类型.
func needsTimeImport(fields []Field) bool {
	for _, f := range fields {
		if f.Type == "time.Time" {
			return true
		}
	}
	return false
}
