// Package optionx 提供泛型函数选项模式的通用实现.
package optionx

// Option 无错误的函数选项类型.
type Option[T any] func(*T)

// OptionErr 可返回错误的函数选项类型.
type OptionErr[T any] func(*T) error

// Apply 依次应用所有选项到目标对象.
func Apply[T any](t *T, opts ...Option[T]) {
	for _, opt := range opts {
		opt(t)
	}
}

// ApplyErr 遇到第一个错误立即返回fail-fast.
func ApplyErr[T any](t *T, opts ...OptionErr[T]) error {
	for _, opt := range opts {
		if err := opt(t); err != nil {
			return err
		}
	}
	return nil
}
