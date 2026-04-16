package compose

import "fmt"

// START 图的起始虚拟节点 key.
const START = "__start__"

// END 图的终止虚拟节点 key.
const END = "__end__"

// NodeError 节点执行错误.
type NodeError struct {
	NodeID string
	Err    error
}

func (e *NodeError) Error() string {
	return fmt.Sprintf("compose: node %q: %v", e.NodeID, e.Err)
}

func (e *NodeError) Unwrap() error { return e.Err }

// CompileError 图编译错误.
type CompileError struct {
	Reason string
}

func (e *CompileError) Error() string {
	return fmt.Sprintf("compose: compile error: %s", e.Reason)
}
