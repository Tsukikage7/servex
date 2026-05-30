package llm

// Usage token 用量统计.
type Usage struct {
	// PromptTokens 输入提示词token 数.
	PromptTokens int
	// CompletionTokens 输出生成token 数.
	CompletionTokens int
	// TotalTokens 总 token 数.
	TotalTokens int
	// CachedTokens 命中缓存的 token 数部分 Provider 支持.
	CachedTokens int
}

// Add 将 other 的用量累加到当前 Usage.
func (u *Usage) Add(other Usage) {
	u.PromptTokens += other.PromptTokens
	u.CompletionTokens += other.CompletionTokens
	u.TotalTokens += other.TotalTokens
	u.CachedTokens += other.CachedTokens
}
