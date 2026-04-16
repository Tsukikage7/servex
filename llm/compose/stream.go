// Package compose 提供 DAG 编排引擎，支持泛型节点组合、条件边、并发调度和流式传播.
package compose

import (
	"io"
	"sync"
)

// StreamReader 泛型流式读取器.
type StreamReader[T any] struct {
	mu     sync.Mutex
	recv   func() (T, error)
	closed bool
}

func newStreamReader[T any](recv func() (T, error)) *StreamReader[T] {
	return &StreamReader[T]{recv: recv}
}

// NewSliceStreamReader 从切片创建流（测试用）.
func NewSliceStreamReader[T any](items []T) *StreamReader[T] {
	pos := 0
	return newStreamReader(func() (T, error) {
		if pos >= len(items) {
			var zero T
			return zero, io.EOF
		}
		v := items[pos]
		pos++
		return v, nil
	})
}

// NewChanStreamReader 从 channel 创建流.
func NewChanStreamReader[T any](ch <-chan T) *StreamReader[T] {
	return newStreamReader(func() (T, error) {
		v, ok := <-ch
		if !ok {
			var zero T
			return zero, io.EOF
		}
		return v, nil
	})
}

// Recv 读取下一个元素，流结束时返回 io.EOF.
func (r *StreamReader[T]) Recv() (T, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		var zero T
		return zero, io.EOF
	}
	return r.recv()
}

// Close 关闭流.
func (r *StreamReader[T]) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return nil
}

// BoxStream 将单个值包装为单帧 StreamReader（boxing）.
func BoxStream[T any](v T) *StreamReader[T] {
	sent := false
	return newStreamReader(func() (T, error) {
		if sent {
			var zero T
			return zero, io.EOF
		}
		sent = true
		return v, nil
	})
}

// ConcatStream 将流中所有帧通过 concat 函数拼接为单个值.
func ConcatStream[T any](r *StreamReader[T], concat func(T, T) T) (T, error) {
	var result T
	first := true
	for {
		v, err := r.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			var zero T
			return zero, err
		}
		if first {
			result = v
			first = false
		} else {
			result = concat(result, v)
		}
	}
	return result, nil
}

// TeeStream 将流复制为两个独立流.
// 注意：第一次 Recv 调用会 eager drain 整个源流到内存缓冲区.
// 适用于源流长度可控的场景；大流量数据请使用其他分发机制.
func TeeStream[T any](r *StreamReader[T]) (*StreamReader[T], *StreamReader[T]) {
	var buf1, buf2 []T
	var mu sync.Mutex
	var done bool
	var drainErr error

	drain := func() {
		mu.Lock()
		defer mu.Unlock()
		if done {
			return
		}
		for {
			v, err := r.Recv()
			if err != nil {
				drainErr = err
				break
			}
			buf1 = append(buf1, v)
			buf2 = append(buf2, v)
		}
		done = true
	}

	makeReader := func(bufPtr *[]T) *StreamReader[T] {
		pos := 0
		return newStreamReader(func() (T, error) {
			drain()
			mu.Lock()
			defer mu.Unlock()
			if pos >= len(*bufPtr) {
				var zero T
				if drainErr == io.EOF {
					return zero, io.EOF
				}
				return zero, drainErr
			}
			v := (*bufPtr)[pos]
			pos++
			return v, nil
		})
	}

	return makeReader(&buf1), makeReader(&buf2)
}
