// Package sorting 提供排序参数处理功能.
package sorting

import (
	"strings"
	"unicode"
)

// Order 排序方向.
type Order string

const (
	// Asc 升序.
	Asc Order = "asc"
	// Desc 降序.
	Desc Order = "desc"
)

// DefaultOrder 默认排序方向.
const DefaultOrder = Desc

// Sort 单个排序条件.
type Sort struct {
	Field string // 排序字段
	Order Order  // 排序方向
}

// String 返回排序字符串，如 "created_time desc".
func (s Sort) String() string {
	return s.Field + " " + string(s.Order)
}

// Sorting 排序参数.
type Sorting struct {
	Sorts []Sort
}

// New 创建排序参数.
// 支持格式:
//   - "field" -> field desc (默认降序)
//   - "field:asc" 或 "field:desc"
//   - "field1:desc,field2:asc" (多字段)
//
// 使用示例:
//
//	sorting.New("created_time")                    // created_time desc
//	sorting.New("name:asc")                        // name asc
//	sorting.New("created_time:desc,id:asc")        // created_time desc, id asc
func New(sort string) Sorting {
	s := Sorting{}
	s.parse(sort)
	return s
}

// parse 解析排序字符串.
func (s *Sorting) parse(sort string) {
	if sort == "" {
		return
	}

	parts := strings.Split(sort, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		field, order := parseField(part)
		if field != "" {
			s.Sorts = append(s.Sorts, Sort{Field: field, Order: order})
		}
	}
}

// parseField 解析单个字段，返回字段名和排序方向.
// 字段名仅允许字母、数字、下划线，防止 ORDER BY 注入.
func parseField(part string) (string, Order) {
	if idx := strings.LastIndex(part, ":"); idx > 0 {
		field := strings.TrimSpace(part[:idx])
		orderStr := strings.ToLower(strings.TrimSpace(part[idx+1:]))

		order := DefaultOrder
		if orderStr == "asc" {
			order = Asc
		} else if orderStr == "desc" {
			order = Desc
		}

		if !isSafeFieldName(field) {
			return "", DefaultOrder
		}
		return field, order
	}
	field := strings.TrimSpace(part)
	if !isSafeFieldName(field) {
		return "", DefaultOrder
	}
	return field, DefaultOrder
}

// isSafeFieldName 校验字段名是否安全（仅允许字母、数字、下划线和点号），防止 SQL 注入.
func isSafeFieldName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '.' {
			return false
		}
	}
	return true
}

// IsEmpty 是否为空.
func (s Sorting) IsEmpty() bool {
	return len(s.Sorts) == 0
}

// First 返回第一个排序条件，如果为空返回零值.
func (s Sorting) First() Sort {
	if len(s.Sorts) > 0 {
		return s.Sorts[0]
	}
	return Sort{}
}

// String 返回完整排序字符串，如 "created_time desc, id asc".
func (s Sorting) String() string {
	if len(s.Sorts) == 0 {
		return ""
	}

	parts := make([]string, len(s.Sorts))
	for i, sort := range s.Sorts {
		parts[i] = sort.String()
	}
	return strings.Join(parts, ", ")
}

// Filter 过滤排序字段，只保留允许的字段（白名单）.
// 使用示例:
//
//	sorting.New("name:asc,password:desc").Filter("id", "name", "created_time")
//	// 结果只保留 name:asc，password 被过滤
func (s Sorting) Filter(allowedFields ...string) Sorting {
	if len(allowedFields) == 0 {
		return s
	}

	allowed := make(map[string]bool, len(allowedFields))
	for _, f := range allowedFields {
		allowed[f] = true
	}

	filtered := Sorting{}
	for _, sort := range s.Sorts {
		if allowed[sort.Field] {
			filtered.Sorts = append(filtered.Sorts, sort)
		}
	}
	return filtered
}

// WithDefault 如果排序为空，使用默认排序.
// 使用示例:
//
//	sorting.New("").WithDefault("created_time:desc")
func (s Sorting) WithDefault(defaultSort string) Sorting {
	if s.IsEmpty() {
		return New(defaultSort)
	}
	return s
}
