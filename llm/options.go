package llm

// CallOption 调用选项函数.
type CallOption func(*CallOptions)

// CallOptions 调用选项集合导出供子包使用.
type CallOptions struct {
	Model       string
	Temperature *float64
	MaxTokens   *int
	TopP        *float64
	Stop        []string
	Tools       []Tool
	ToolChoice  *ToolChoice
	StreamFunc  StreamCallback
}

// ApplyOptions 应用选项列表，返回合并后的选项.
func ApplyOptions(opts []CallOption) CallOptions {
	var o CallOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// WithModel 指定模型名称覆盖 Provider 默认模型.
func WithModel(model string) CallOption {
	return func(o *CallOptions) { o.Model = model }
}

// WithTemperature 设置采样温度0.0~2.0，越高越随机.
func WithTemperature(t float64) CallOption {
	return func(o *CallOptions) { o.Temperature = &t }
}

// WithMaxTokens 设置最大生成 token 数.
func WithMaxTokens(n int) CallOption {
	return func(o *CallOptions) { o.MaxTokens = &n }
}

// WithTopP 设置 nucleus sampling 参数0.0~1.0.
func WithTopP(p float64) CallOption {
	return func(o *CallOptions) { o.TopP = &p }
}

// WithStop 设置停止词，遇到这些词时停止生成.
func WithStop(stop ...string) CallOption {
	return func(o *CallOptions) { o.Stop = stop }
}

// WithTools 设置可用工具列表.
func WithTools(tools ...Tool) CallOption {
	return func(o *CallOptions) { o.Tools = tools }
}

// WithToolChoice 设置工具选择策略.
func WithToolChoice(choice ToolChoice) CallOption {
	return func(o *CallOptions) { o.ToolChoice = &choice }
}

// WithStreamCallback 设置流式回调函数在 Generate 中使用，边生成边回调.
func WithStreamCallback(fn StreamCallback) CallOption {
	return func(o *CallOptions) { o.StreamFunc = fn }
}
