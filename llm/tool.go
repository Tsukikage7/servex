package llm

import "encoding/json"

// Tool 工具定义.
type Tool struct {
	// Function 函数定义.
	Function FunctionDef
}

// FunctionDef 函数定义.
type FunctionDef struct {
	// Name 函数名称.
	Name string
	// Description 函数描述，帮助模型理解何时调用此函数.
	Description string
	// Parameters JSON Schema 格式的参数定义.
	Parameters json.RawMessage
}

// ToolCall 工具调用请求.
type ToolCall struct {
	// ID 工具调用 ID，用于匹配结果.
	ID string
	// Function 调用的函数信息.
	Function struct {
		// Name 函数名称.
		Name string
		// Arguments JSON 格式的参数.
		Arguments string
	}
}

// ToolChoice 工具选择策略.
type ToolChoice struct {
	// Type 选择类型："auto", "none", "required", "function".
	Type string
	// Function 指定必须调用的函数Type="function" 时.
	Function *struct{ Name string }
}

// 预定义工具选择策略.
var (
	// ToolChoiceAuto 让模型自动决定是否调用工具.
	ToolChoiceAuto = ToolChoice{Type: "auto"}
	// ToolChoiceNone 禁止调用工具.
	ToolChoiceNone = ToolChoice{Type: "none"}
	// ToolChoiceRequired 强制调用工具.
	ToolChoiceRequired = ToolChoice{Type: "required"}
)

// ToolChoiceFunction 指定必须调用的函数名.
func ToolChoiceFunction(name string) ToolChoice {
	return ToolChoice{
		Type:     "function",
		Function: &struct{ Name string }{Name: name},
	}
}
