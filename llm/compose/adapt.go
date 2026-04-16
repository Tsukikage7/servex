package compose

import (
	"context"
	"fmt"
	"reflect"
)

// callMode 控制图执行时节点的调用范式.
type callMode int

const (
	// modeInvoke: Invoke 调用——所有节点以 Invoke 范式执行.
	// Stream 节点输出自动 ConcatStream 为单值；Collect/Transform 节点输入自动 BoxStream.
	modeInvoke callMode = iota

	// modeStreaming: Stream/Collect/Transform 调用——所有节点以 Transform 范式执行.
	// Invoke 节点输出自动 BoxStream 为单帧流；Collect 节点输出也自动 BoxStream.
	modeStreaming
)

// adaptedExecNode 根据 callMode 适配节点执行.
func adaptedExecNode(ctx context.Context, nf nodeFunc, input any, mode callMode) (any, error) {
	switch mode {
	case modeInvoke:
		return execInvokeMode(ctx, nf, input)
	case modeStreaming:
		return execStreamingMode(ctx, nf, input)
	default:
		return callNodeFunc(ctx, nf, input)
	}
}

// execInvokeMode 在 Invoke 模式下执行节点，最终输出单值（非流）.
func execInvokeMode(ctx context.Context, nf nodeFunc, input any) (any, error) {
	switch nf.kind {
	case kindInvoke:
		// 直接调用，已是单值输出
		return callNodeFunc(ctx, nf, input)

	case kindStream:
		// 调用后通过 concatStreamFn + concatFn 将流输出 concat 为单值
		raw, err := callNodeFunc(ctx, nf, input)
		if err != nil {
			return nil, err
		}
		return applyConcat(raw, nf)

	case kindCollect:
		// 将单值输入 BoxStream，再调用节点
		boxed, err := applyBoxInput(input, nf)
		if err != nil {
			return nil, err
		}
		return callNodeFunc(ctx, nf, boxed)

	case kindTransform:
		// BoxStream 输入 + ConcatStream 输出
		boxed, err := applyBoxInput(input, nf)
		if err != nil {
			return nil, err
		}
		raw, err := callNodeFunc(ctx, nf, boxed)
		if err != nil {
			return nil, err
		}
		return applyConcat(raw, nf)

	default:
		return callNodeFunc(ctx, nf, input)
	}
}

// execStreamingMode 在 Streaming 模式下执行节点，最终输出流（*StreamReader[T]）.
func execStreamingMode(ctx context.Context, nf nodeFunc, input any) (any, error) {
	switch nf.kind {
	case kindInvoke:
		// 调用后将单值输出 BoxStream 为单帧流
		raw, err := callNodeFunc(ctx, nf, input)
		if err != nil {
			return nil, err
		}
		return applyBoxOutput(raw, nf)

	case kindStream:
		// 已是流式输出，直接调用
		return callNodeFunc(ctx, nf, input)

	case kindCollect:
		// 接受流输入，返回单值 → BoxStream 输出
		raw, err := callNodeFunc(ctx, nf, input)
		if err != nil {
			return nil, err
		}
		return applyBoxOutput(raw, nf)

	case kindTransform:
		// 直接调用：流输入 → 流输出
		return callNodeFunc(ctx, nf, input)

	default:
		return callNodeFunc(ctx, nf, input)
	}
}

// applyBoxInput 将单值 input 通过 boxInputStream 包装为 *StreamReader[I].
// boxInputStream 是 func(I) *StreamReader[I]，在节点构造时预先捕获.
func applyBoxInput(input any, nf nodeFunc) (boxed any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("compose: boxing input failed: %v", r)
		}
	}()
	if nf.boxInputStream == nil {
		return nil, fmt.Errorf("compose: node kind=%d missing boxInputStream helper", nf.kind)
	}
	results := reflect.ValueOf(nf.boxInputStream).Call([]reflect.Value{reflect.ValueOf(input)})
	return results[0].Interface(), nil
}

// applyBoxOutput 将单值 raw 通过 boxFn 包装为 *StreamReader[O].
// boxFn 是 func(O) *StreamReader[O]，在节点构造时预先捕获.
func applyBoxOutput(raw any, nf nodeFunc) (boxed any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("compose: boxing output failed: %v", r)
		}
	}()
	if nf.boxFn == nil {
		return nil, fmt.Errorf("compose: node kind=%d missing boxFn helper", nf.kind)
	}
	results := reflect.ValueOf(nf.boxFn).Call([]reflect.Value{reflect.ValueOf(raw)})
	return results[0].Interface(), nil
}

// applyConcat 将流式输出 raw (*StreamReader[O]) 通过 concatStreamFn + concatFn concat 为单值.
// concatStreamFn 是 func(*StreamReader[O], func(O,O)O) (O, error)，在节点构造时预先捕获.
func applyConcat(raw any, nf nodeFunc) (result any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("compose: concat stream failed: %v", r)
		}
	}()
	if nf.concatStreamFn == nil {
		return nil, fmt.Errorf("compose: stream node missing concatStreamFn; use WithConcatFunc to register")
	}
	if nf.concatFn == nil {
		return nil, fmt.Errorf("compose: stream node missing concatFn; use WithConcatFunc to register")
	}
	results := reflect.ValueOf(nf.concatStreamFn).Call([]reflect.Value{
		reflect.ValueOf(raw),
		reflect.ValueOf(nf.concatFn),
	})
	if !results[1].IsNil() {
		return nil, results[1].Interface().(error)
	}
	return results[0].Interface(), nil
}
