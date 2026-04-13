// Package structured 提供结构化输出提取功能，将 LLM 约束为输出特定 Go struct.
package structured

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/Tsukikage7/servex/v2/llm"
)

// options 内部选项集合.
type options struct {
	maxRetries        int
	callOptions       []llm.CallOption
	schemaDescription string
}

// defaultOptions 返回默认选项.
func defaultOptions() *options {
	return &options{
		maxRetries: 3,
	}
}

// Option 选项函数.
type Option func(*options)

// WithMaxRetries 设置 JSON 解析失败时的最大重试次数（默认 3）.
func WithMaxRetries(n int) Option {
	return func(o *options) { o.maxRetries = n }
}

// WithCallOptions 设置底层模型调用选项.
func WithCallOptions(opts ...llm.CallOption) Option {
	return func(o *options) { o.callOptions = append(o.callOptions, opts...) }
}

// WithSchemaDescription 设置 Schema 顶层描述.
func WithSchemaDescription(desc string) Option {
	return func(o *options) { o.schemaDescription = desc }
}

// Extract 从 LLM 响应中提取结构化数据.
// 自动生成 JSON Schema 并注入系统提示，解析失败时自动重试.
func Extract[T any](ctx context.Context, model llm.ChatModel, prompt string, opts ...Option) (T, error) {
	messages := []llm.Message{llm.UserMessage(prompt)}
	return ExtractFromMessages[T](ctx, model, messages, opts...)
}

// ExtractFromMessages 从已有消息列表中提取结构化数据.
func ExtractFromMessages[T any](ctx context.Context, model llm.ChatModel, messages []llm.Message, opts ...Option) (T, error) {
	var zero T
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	// 生成 JSON Schema.
	schema := SchemaFrom[T]()
	if o.schemaDescription != "" {
		// 将 description 注入到 schema 顶层.
		var m map[string]any
		if err := json.Unmarshal(schema, &m); err == nil {
			m["description"] = o.schemaDescription
			if b, err := json.Marshal(m); err == nil {
				schema = b
			}
		}
	}

	// 构造系统消息.
	sysMsg := llm.SystemMessage(fmt.Sprintf(
		"请严格按以下 JSON Schema 输出，不要输出其他内容：\n%s",
		string(schema),
	))

	// 在消息列表前插入系统消息.
	allMessages := make([]llm.Message, 0, len(messages)+1)
	allMessages = append(allMessages, sysMsg)
	allMessages = append(allMessages, messages...)

	// 重试循环.
	for attempt := 0; attempt <= o.maxRetries; attempt++ {
		resp, err := model.Generate(ctx, allMessages, o.callOptions...)
		if err != nil {
			return zero, fmt.Errorf("structured: 模型调用失败: %w", err)
		}

		content := resp.Message.Content

		// 尝试提取 JSON（支持模型在代码块中包裹的情况）.
		content = llm.ExtractJSON(content)

		var result T
		if err := json.Unmarshal([]byte(content), &result); err == nil {
			return result, nil
		} else if attempt < o.maxRetries {
			// 将错误和模型输出追加到消息历史，引导模型修正.
			allMessages = append(allMessages,
				llm.AssistantMessage(resp.Message.Content),
				llm.UserMessage(fmt.Sprintf(
					"输出的 JSON 无法解析：%v\n请严格按 JSON Schema 重新输出，只输出 JSON，不要任何额外内容。",
					err,
				)),
			)
		} else {
			return zero, fmt.Errorf("structured: JSON 解析失败（已重试 %d 次）: %w", o.maxRetries, err)
		}
	}

	return zero, fmt.Errorf("structured: 超出最大重试次数 %d", o.maxRetries)
}

// SchemaFrom 从 Go struct 类型参数生成 JSON Schema.
// 支持 string、int、float、bool、slice 及嵌套 struct.
// 使用 `json` tag 作为字段名，`description` tag 作为字段描述.
func SchemaFrom[T any]() json.RawMessage {
	var zero T
	t := reflect.TypeOf(zero)
	schema := buildSchema(t)
	b, _ := json.Marshal(schema)
	return json.RawMessage(b)
}

// buildSchema 递归构建 JSON Schema map.
func buildSchema(t reflect.Type) map[string]any {
	// 解引用指针.
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice:
		return map[string]any{
			"type":  "array",
			"items": buildSchema(t.Elem()),
		}
	case reflect.Struct:
		return buildStructSchema(t)
	default:
		return map[string]any{"type": "string"}
	}
}

// buildStructSchema 构建 struct 类型的 JSON Schema.
func buildStructSchema(t reflect.Type) map[string]any {
	properties := map[string]any{}
	required := []string{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// 跳过非导出字段.
		if !field.IsExported() {
			continue
		}

		// 获取 json tag 作为字段名.
		fieldName := field.Name
		if tag := field.Tag.Get("json"); tag != "" {
			parts := strings.Split(tag, ",")
			if parts[0] != "" && parts[0] != "-" {
				fieldName = parts[0]
			} else if parts[0] == "-" {
				continue
			}
		}

		// 递归构建字段的 schema.
		fieldSchema := buildSchema(field.Type)

		// 添加 description tag.
		if desc := field.Tag.Get("description"); desc != "" {
			fieldSchema["description"] = desc
		}

		properties[fieldName] = fieldSchema
		required = append(required, fieldName)
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}
