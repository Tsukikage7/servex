package compose

// WithConcatFunc 为 Lambda 节点注册 concat 函数.
// 当 Invoke 模式下该节点产生流式输出时，框架用此函数将流拼接为单值.
// concatFn 签名：func(accumulator T, next T) T
func WithConcatFunc[T any](nf nodeFunc, concatFn func(T, T) T) nodeFunc {
	nf.concatFn = concatFn
	// 同时更新 concatStreamFn，使其使用新的 concatFn
	nf.concatStreamFn = func(r *StreamReader[T], concat func(T, T) T) (T, error) {
		return ConcatStream(r, concat)
	}
	return nf
}
