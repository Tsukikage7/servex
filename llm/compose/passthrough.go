package compose

import "context"

// Passthrough 创建直通节点（输入即输出，用于并行分支对齐）.
func Passthrough[T any]() nodeFunc {
	return InvokableLambda(func(_ context.Context, v T) (T, error) {
		return v, nil
	})
}
