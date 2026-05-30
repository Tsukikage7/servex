package config

import "strings"

// Options 配置加载选项.
type Options struct {
	// EnvPrefix 环境变量前缀，例如 "APP" 会将 APP_DATABASE_HOST 映射到 database.host
	EnvPrefix string

	// EnvKeyReplacer 环境变量键替换器，默认将 . 替换为 _
	EnvKeyReplacer *strings.Replacer

	// AutomaticEnv 是否自动绑定环境变量
	AutomaticEnv bool

	// AllowEmptyEnv 是否允许空环境变量值覆盖配置
	AllowEmptyEnv bool

	// ConfigType 显式指定配置文件类型yaml, json, toml 等
	ConfigType string

	// Defaults 默认配置值
	Defaults map[string]any
}

// DefaultOptions 返回默认选项.
func DefaultOptions() *Options {
	return &Options{
		EnvKeyReplacer: strings.NewReplacer(".", "_"),
		AutomaticEnv:   true,
		AllowEmptyEnv:  false,
	}
}

// Option 配置选项函数.
type Option func(*Options)

// WithEnvPrefix 设置环境变量前缀.
func WithEnvPrefix(prefix string) Option {
	return func(o *Options) {
		o.EnvPrefix = prefix
	}
}

// WithAutomaticEnv 启用自动环境变量绑定.
func WithAutomaticEnv() Option {
	return func(o *Options) {
		o.AutomaticEnv = true
	}
}

// WithDefaults 设置默认值.
func WithDefaults(defaults map[string]any) Option {
	return func(o *Options) {
		o.Defaults = defaults
	}
}

// WithConfigType 显式指定配置文件类型.
func WithConfigType(configType string) Option {
	return func(o *Options) {
		o.ConfigType = configType
	}
}
