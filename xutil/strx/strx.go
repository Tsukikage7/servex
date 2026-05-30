// Package strx 提供字符串工具函数.
package strx

import (
	"strings"
	"unicode"
	"unsafe"
)

// SplitName 将全名拆分为 firstName 和 lastName.
func SplitName(fullName string) (string, string) {
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return "", ""
	}

	parts := strings.SplitN(fullName, " ", 2)
	firstName := strings.TrimSpace(parts[0])

	var lastName string
	if len(parts) > 1 {
		lastName = strings.TrimSpace(parts[1])
	}

	return firstName, lastName
}

// JoinName 将姓和名合并为全名.
func JoinName(firstName, lastName string) string {
	firstName = strings.TrimSpace(firstName)
	lastName = strings.TrimSpace(lastName)

	if firstName == "" && lastName == "" {
		return ""
	}
	if firstName == "" {
		return lastName
	}
	if lastName == "" {
		return firstName
	}

	return firstName + " " + lastName
}

// TrimAndLower 去除前后空格并转为小写.
func TrimAndLower(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// TrimAndUpper 去除前后空格并转为大写.
func TrimAndUpper(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// IsEmpty 检查字符串是否为空仅含空白字符视为空.
func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// IsNotEmpty 检查字符串是否非空.
func IsNotEmpty(s string) bool {
	return !IsEmpty(s)
}

// ToTitle 将字符串转为标题格式首字母大写，其余小写.
func ToTitle(s string) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) == 0 {
		return ""
	}

	runes[0] = unicode.ToUpper(runes[0])
	for i := 1; i < len(runes); i++ {
		runes[i] = unicode.ToLower(runes[i])
	}

	return string(runes)
}

// Truncate 截断字符串到指定长度，超出部分用省略号替代.
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}

	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}

	if maxLen <= 3 {
		return string(runes[:maxLen])
	}

	return string(runes[:maxLen-3]) + "..."
}

// DefaultIfEmpty 如果字符串为空则返回默认值.
func DefaultIfEmpty(s, defaultValue string) string {
	if IsEmpty(s) {
		return defaultValue
	}
	return s
}

// UnsafeToBytes 零分配地将 string 转换为 []byte.
//
// UNSAFE: 返回的 []byte 共享底层内存，绝对不可修改，否则行为未定义可能导致崩溃或数据损坏.
// 仅在性能关键路径且确保不会修改返回值时使用.
func UnsafeToBytes(s string) []byte {
	if s == "" {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// UnsafeToString 零分配地将 []byte 转换为 string.
//
// UNSAFE: 转换后不应再修改原 []byte，否则行为未定义可能导致崩溃或数据损坏.
// 仅在性能关键路径且确保不会修改原切片时使用.
func UnsafeToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}
