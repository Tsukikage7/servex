package openapi

import (
	"reflect"
	"strconv"
	"strings"
	"time"
)

// SchemaFrom 从 Go 值通过反射生成 JSON Schema.
// 支持 json、validate、description 三种 struct tag.
func SchemaFrom(v any) *Schema {
	return schemaFromType(reflect.TypeOf(v))
}

func schemaFromType(t reflect.Type) *Schema {
	if t == nil {
		return &Schema{Type: "object"}
	}

	// 解引用指针
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// 特殊类型
	if t == reflect.TypeOf(time.Time{}) {
		return &Schema{Type: "string", Format: "date-time"}
	}

	switch t.Kind() {
	case reflect.String:
		return &Schema{Type: "string"}
	case reflect.Bool:
		return &Schema{Type: "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &Schema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return &Schema{Type: "number"}
	case reflect.Slice, reflect.Array:
		return &Schema{
			Type:  "array",
			Items: schemaFromType(t.Elem()),
		}
	case reflect.Map:
		return &Schema{Type: "object"}
	case reflect.Struct:
		return structSchema(t)
	default:
		return &Schema{Type: "object"}
	}
}

func structSchema(t reflect.Type) *Schema {
	s := &Schema{
		Type:       "object",
		Properties: make(map[string]*Schema),
	}

	for i := range t.NumField() {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		// json tag 解析字段名
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		name, jsonOpts := parseTag(jsonTag)
		if name == "" {
			name = field.Name
		}

		// 生成字段 schema
		fieldSchema := schemaFromType(field.Type)

		// description tag
		if desc := field.Tag.Get("description"); desc != "" {
			fieldSchema.Description = desc
		}

		// validate tag 解析约束
		validateTag := field.Tag.Get("validate")
		if validateTag != "" {
			parseValidateTag(validateTag, fieldSchema)
			if strings.Contains(validateTag, "required") && !strings.Contains(jsonOpts, "omitempty") {
				s.Required = append(s.Required, name)
			}
		}

		s.Properties[name] = fieldSchema
	}

	return s
}

func parseTag(tag string) (name string, opts string) {
	parts := strings.SplitN(tag, ",", 2)
	name = parts[0]
	if len(parts) > 1 {
		opts = parts[1]
	}
	return
}

func parseValidateTag(tag string, s *Schema) {
	parts := strings.Split(tag, ",")
	for _, part := range parts {
		if strings.HasPrefix(part, "min=") {
			valStr := part[4:]
			if s.Type == "string" {
				// 字符串类型使用 minLength
				if v, err := strconv.Atoi(valStr); err == nil {
					s.MinLength = &v
				}
			} else {
				// 数值类型使用 minimum
				if v, err := strconv.ParseFloat(valStr, 64); err == nil {
					s.Minimum = &v
				}
			}
		}
		if strings.HasPrefix(part, "max=") {
			valStr := part[4:]
			if s.Type == "string" {
				// 字符串类型使用 maxLength
				if v, err := strconv.Atoi(valStr); err == nil {
					s.MaxLength = &v
				}
			} else {
				// 数值类型使用 maximum
				if v, err := strconv.ParseFloat(valStr, 64); err == nil {
					s.Maximum = &v
				}
			}
		}
	}
}
