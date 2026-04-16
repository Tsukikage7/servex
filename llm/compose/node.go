package compose

import (
	"context"
	"fmt"
	"reflect"
)

type nodeKind int

const (
	kindInvoke nodeKind = iota
	kindStream
	kindCollect
	kindTransform
)

// nodeFunc 节点内部表示.
type nodeFunc struct {
	kind       nodeKind
	fn         any
	inputType  reflect.Type
	outputType reflect.Type

	// concatFn: func(T, T) T — 用于 Invoke 模式下将流输出 concat 为单值.
	// 由 WithConcatFunc 设置，或在构造 kindStream/kindTransform 节点时通过内置默认值填充.
	concatFn any

	// boxFn: func(O) *StreamReader[O] — Streaming 模式下将 Invoke/Collect 输出包装为流.
	// 在 Lambda 构造函数中预先捕获，避免泛型擦除后无法调用.
	boxFn any

	// concatStreamFn: func(*StreamReader[O], func(O,O)O) (O, error) — Invoke 模式下消费流输出.
	// 在 Lambda 构造函数中预先捕获.
	concatStreamFn any

	// boxInputStream: func(I) *StreamReader[I] — Invoke 模式下将单值输入包装为流，给 Collect/Transform 节点使用.
	// 在 Lambda 构造函数中预先捕获.
	boxInputStream any
}

// InvokableLambda 创建 Ping-Pong 节点（同步输入，同步输出）.
func InvokableLambda[I, O any](fn func(context.Context, I) (O, error)) nodeFunc {
	var zeroI I
	var zeroO O
	return nodeFunc{
		kind:       kindInvoke,
		fn:         fn,
		inputType:  reflect.TypeOf(&zeroI).Elem(),
		outputType: reflect.TypeOf(&zeroO).Elem(),
		boxFn:      func(v O) *StreamReader[O] { return BoxStream(v) },
	}
}

// StreamableLambda 创建 Server-Streaming 节点（同步输入，流式输出）.
func StreamableLambda[I, O any](fn func(context.Context, I) (*StreamReader[O], error)) nodeFunc {
	var zeroI I
	var zeroO O
	return nodeFunc{
		kind:           kindStream,
		fn:             fn,
		inputType:      reflect.TypeOf(&zeroI).Elem(),
		outputType:     reflect.TypeOf(&zeroO).Elem(),
		concatStreamFn: func(r *StreamReader[O], concat func(O, O) O) (O, error) { return ConcatStream(r, concat) },
	}
}

// CollectableLambda 创建 Client-Streaming 节点（流式输入，同步输出）.
func CollectableLambda[I, O any](fn func(context.Context, *StreamReader[I]) (O, error)) nodeFunc {
	var zeroI I
	var zeroO O
	return nodeFunc{
		kind:           kindCollect,
		fn:             fn,
		inputType:      reflect.TypeOf(&zeroI).Elem(),
		outputType:     reflect.TypeOf(&zeroO).Elem(),
		boxInputStream: func(v I) *StreamReader[I] { return BoxStream(v) },
		boxFn:          func(v O) *StreamReader[O] { return BoxStream(v) },
	}
}

// TransformableLambda 创建 Bidirectional-Streaming 节点（流式输入，流式输出）.
func TransformableLambda[I, O any](fn func(context.Context, *StreamReader[I]) (*StreamReader[O], error)) nodeFunc {
	var zeroI I
	var zeroO O
	return nodeFunc{
		kind:           kindTransform,
		fn:             fn,
		inputType:      reflect.TypeOf(&zeroI).Elem(),
		outputType:     reflect.TypeOf(&zeroO).Elem(),
		boxInputStream: func(v I) *StreamReader[I] { return BoxStream(v) },
		concatStreamFn: func(r *StreamReader[O], concat func(O, O) O) (O, error) { return ConcatStream(r, concat) },
	}
}

// callNodeFunc 通过反射调用节点函数.
// input 可以是任何类型，函数签名必须为 func(context.Context, T) (R, error).
func callNodeFunc(ctx context.Context, nf nodeFunc, input any) (any, error) {
	fn := reflect.ValueOf(nf.fn)
	// nil 保护：当 input 为 untyped nil 时，构造对应类型的零值
	var inputVal reflect.Value
	if input == nil {
		inputVal = reflect.New(nf.inputType).Elem()
	} else {
		inputVal = reflect.ValueOf(input)
	}
	results := fn.Call([]reflect.Value{reflect.ValueOf(ctx), inputVal})
	if !results[1].IsNil() {
		return nil, results[1].Interface().(error)
	}
	return results[0].Interface(), nil
}

func validateNodeFunc(nf nodeFunc) error {
	if nf.fn == nil {
		return fmt.Errorf("node function is nil")
	}
	return nil
}
