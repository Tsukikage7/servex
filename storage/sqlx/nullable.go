// Package sqlx 提供 SQL 类型辅助工具，包括泛型 Nullable 包装.
package sqlx

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"
)

// Nullable 泛型 nullable 包装，支持 JSON/SQL 双向序列化.
// Val 存储实际值，Valid 为 false 时表示 NULL.
type Nullable[T any] struct {
	Val   T
	Valid bool
}

// Of 创建 Valid=true 的 Nullable.
func Of[T any](v T) Nullable[T] {
	return Nullable[T]{Val: v, Valid: true}
}

// Null 创建 Valid=false 的 Nullable表示 NULL.
func Null[T any]() Nullable[T] {
	return Nullable[T]{}
}

// ValueOr 若 Valid 为 false 返回 def，否则返回 Val.
func (n Nullable[T]) ValueOr(def T) T {
	if !n.Valid {
		return def
	}
	return n.Val
}

// MarshalJSON 实现 json.Marshaler 接口.
// Valid=false 时序列化为 null，否则序列化 Val.
func (n Nullable[T]) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.Val)
}

// UnmarshalJSON 实现 json.Unmarshaler 接口.
// JSON null 解析为 Valid=false，其他值解析为 Val 并设 Valid=true.
func (n *Nullable[T]) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Valid = false
		return nil
	}
	if err := json.Unmarshal(data, &n.Val); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

// Value 实现 driver.Valuer 接口，用于写入数据库.
// Valid=false 时返回 nilNULL，否则通过反射提取基础类型值.
func (n Nullable[T]) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	// 尝试直接转换为 driver.Value 支持的类型
	v := any(n.Val)
	switch val := v.(type) {
	case int64:
		return val, nil
	case float64:
		return val, nil
	case bool:
		return val, nil
	case []byte:
		return val, nil
	case string:
		return val, nil
	case driver.Valuer:
		return val.Value()
	default:
		// 通过反射处理衍生类型如 ~int64 等
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return rv.Int(), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return int64(rv.Uint()), nil
		case reflect.Float32, reflect.Float64:
			return rv.Float(), nil
		case reflect.Bool:
			return rv.Bool(), nil
		case reflect.String:
			return rv.String(), nil
		case reflect.Slice:
			if rv.Type().Elem().Kind() == reflect.Uint8 {
				return rv.Bytes(), nil
			}
		}
		return nil, fmt.Errorf("sqlx: unsupported Nullable type %T", n.Val)
	}
}

// Scan 实现 sql.Scanner 接口，用于从数据库读取.
// src 为 nil 时设 Valid=false，否则通过标准库 ConvertAssign 转换并设 Valid=true.
func (n *Nullable[T]) Scan(src any) error {
	if src == nil {
		n.Valid = false
		return nil
	}
	if err := convertAssign(&n.Val, src); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

// convertAssign 使用 database/sql 的转换逻辑将 src 赋值给 dest.
func convertAssign(dest any, src any) error {
	// 使用 sql.Null[T] 借助标准库的转换逻辑
	scanner, ok := dest.(sql.Scanner)
	if ok {
		return scanner.Scan(src)
	}
	// 通过反射进行简单赋值
	dv := reflect.ValueOf(dest)
	if dv.Kind() != reflect.Ptr || dv.IsNil() {
		return fmt.Errorf("sqlx: dest must be a non-nil pointer")
	}
	sv := reflect.ValueOf(src)
	dv = dv.Elem()
	if sv.Type().AssignableTo(dv.Type()) {
		dv.Set(sv)
		return nil
	}
	if sv.Type().ConvertibleTo(dv.Type()) {
		dv.Set(sv.Convert(dv.Type()))
		return nil
	}
	return fmt.Errorf("sqlx: cannot convert %T to %T", src, dest)
}

// NullableString 将 sql.NullString 转换为 Nullable[string].
func NullableString(s sql.NullString) Nullable[string] {
	if !s.Valid {
		return Null[string]()
	}
	return Of(s.String)
}

// NullableInt64 将 sql.NullInt64 转换为 Nullable[int64].
func NullableInt64(i sql.NullInt64) Nullable[int64] {
	if !i.Valid {
		return Null[int64]()
	}
	return Of(i.Int64)
}
