package compose

import "context"

type stateKey struct{}

// StateFactory 每次 Graph 执行时创建新 State 实例的工厂函数.
type StateFactory[S any] func() S

func injectState[S any](ctx context.Context, s S) context.Context {
	return context.WithValue(ctx, stateKey{}, s)
}

// GetState 从 context 中取出 State.
func GetState[S any](ctx context.Context) (S, bool) {
	v := ctx.Value(stateKey{})
	if v == nil {
		var zero S
		return zero, false
	}
	s, ok := v.(S)
	return s, ok
}
