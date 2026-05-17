package rdbms

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// JsonValue 数据库非空 JSON 列类型，不同驱动自动映射为 JSON/JSONB/TEXT.
type JsonValue[T any] struct {
	Val T
}

// NewJsonValue 创建非空 JSON 列值.
func NewJsonValue[T any](val T) JsonValue[T] {
	return JsonValue[T]{Val: val}
}

func (j JsonValue[T]) Value() (driver.Value, error) {
	data, err := json.Marshal(j.Val)
	if err != nil {
		return nil, fmt.Errorf("database: json 序列化失败: %w", err)
	}
	return string(data), nil
}

func (j *JsonValue[T]) Scan(src any) error {
	if src == nil {
		var zero T
		j.Val = zero
		return nil
	}

	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("database: json 列不支持类型 %T", src)
	}

	if err := json.Unmarshal(data, &j.Val); err != nil {
		return fmt.Errorf("database: json 反序列化失败: %w", err)
	}
	return nil
}

func (JsonValue[T]) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case DriverMySQL:
		return "JSON"
	case DriverPostgres, DriverPostgreSQL:
		return "JSONB"
	default:
		return "TEXT"
	}
}

// JsonSlice 数据库非空 JSON 数组列类型，nil 会按空数组存储。
type JsonSlice[T any] []T

func (j JsonSlice[T]) Value() (driver.Value, error) {
	if j == nil {
		return "[]", nil
	}
	data, err := json.Marshal(j)
	if err != nil {
		return nil, fmt.Errorf("database: json 序列化失败: %w", err)
	}
	return string(data), nil
}

func (j *JsonSlice[T]) Scan(src any) error {
	if src == nil {
		*j = JsonSlice[T]{}
		return nil
	}

	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("database: json 列不支持类型 %T", src)
	}

	if err := json.Unmarshal(data, j); err != nil {
		return fmt.Errorf("database: json 反序列化失败: %w", err)
	}
	if *j == nil {
		*j = JsonSlice[T]{}
	}
	return nil
}

func (JsonSlice[T]) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case DriverMySQL:
		return "JSON"
	case DriverPostgres, DriverPostgreSQL:
		return "JSONB"
	default:
		return "TEXT"
	}
}
