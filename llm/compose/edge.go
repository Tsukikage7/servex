package compose

import (
	"context"
	"fmt"
	"reflect"
)

type edge struct {
	from string
	to   string
}

// Branch 条件边，根据节点输出选择下一个目标节点.
type Branch struct {
	condFn   any
	condType reflect.Type
	targets  []string
}

// NewBranch 创建条件边.
func NewBranch[T any](condFn func(context.Context, T) (string, error), targets ...string) *Branch {
	var zero T
	return &Branch{
		condFn:   condFn,
		condType: reflect.TypeOf(&zero).Elem(),
		targets:  targets,
	}
}

// Targets 返回所有可能目标节点列表.
func (b *Branch) Targets() []string { return b.targets }

// Evaluate 调用条件函数，返回选中的目标节点 key.
func (b *Branch) Evaluate(ctx context.Context, output any) (string, error) {
	fn := reflect.ValueOf(b.condFn)
	results := fn.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(output)})
	if !results[1].IsNil() {
		return "", results[1].Interface().(error)
	}
	target, ok := results[0].Interface().(string)
	if !ok {
		return "", fmt.Errorf("compose: branch condition returned non-string")
	}
	return target, nil
}

type branchEntry struct {
	from   string
	branch *Branch
}
