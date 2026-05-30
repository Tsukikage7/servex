package oauth2

// AuthURLOptions 存储 AuthURL 的额外参数.
type AuthURLOptions struct {
	Scopes []string
	Prompt string
}

// AuthURLOption 控制 AuthURL 的行为.
type AuthURLOption func(*AuthURLOptions)

// WithExtraScopes 追加额外的 scope.
func WithExtraScopes(scopes ...string) AuthURLOption {
	return func(o *AuthURLOptions) { o.Scopes = append(o.Scopes, scopes...) }
}

// WithPrompt 设置 prompt 参数如 "consent"、"login".
func WithPrompt(prompt string) AuthURLOption {
	return func(o *AuthURLOptions) { o.Prompt = prompt }
}

// ApplyAuthURLOptions 应用选项并返回结果.
func ApplyAuthURLOptions(opts []AuthURLOption) AuthURLOptions {
	var o AuthURLOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
