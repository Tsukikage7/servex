package llm

import "context"

// contextKey context 键类型，避免键冲突.
type contextKey string

const modelNameKey contextKey = "llm:model_name"

// WithModelName 将模型名称注入 context.
func WithModelName(ctx context.Context, model string) context.Context {
	return context.WithValue(ctx, modelNameKey, model)
}

// ModelName 从 context 获取模型名称.
// 未设置时返回空字符串.
func ModelName(ctx context.Context) string {
	if v, ok := ctx.Value(modelNameKey).(string); ok {
		return v
	}
	return ""
}
