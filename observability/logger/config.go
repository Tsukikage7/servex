package logger

import (
	"fmt"
	"strings"
)

// Config 日志配置.
type Config struct {
	// 基础配置
	Type        string `json:"type" toml:"type" yaml:"type" mapstructure:"type"`
	ServiceName string `json:"service_name" toml:"service_name" yaml:"service_name" mapstructure:"service_name"`
	Level       string `json:"level" toml:"level" yaml:"level" mapstructure:"level"`
	Format      string `json:"format" toml:"format" yaml:"format" mapstructure:"format"`

	// 输出配置
	Output         string `json:"output" toml:"output" yaml:"output" mapstructure:"output"`
	LogDir         string `json:"log_dir" toml:"log_dir" yaml:"log_dir" mapstructure:"log_dir"`
	LevelSeparate  bool   `json:"level_separate" toml:"level_separate" yaml:"level_separate" mapstructure:"level_separate"`
	ConsoleEnabled bool   `json:"console_enabled" toml:"console_enabled" yaml:"console_enabled" mapstructure:"console_enabled"`

	// 轮转配置
	RotationEnabled bool   `json:"rotation_enabled" toml:"rotation_enabled" yaml:"rotation_enabled" mapstructure:"rotation_enabled"`
	RotationTime    string `json:"rotation_time" toml:"rotation_time" yaml:"rotation_time" mapstructure:"rotation_time"`
	MaxAge          int    `json:"max_age" toml:"max_age" yaml:"max_age" mapstructure:"max_age"`
	Compress        bool   `json:"compress" toml:"compress" yaml:"compress" mapstructure:"compress"`
	// Timezone 日志文件轮转使用的时区，格式为 IANA 时区名如 "Asia/Shanghai"、"America/Los_Angeles".
	// 留空默认使用 UTC，推荐显式配置以避免部署环境差异.
	Timezone string `json:"timezone" toml:"timezone" yaml:"timezone" mapstructure:"timezone"`

	// 调用者信息配置
	EnableCaller     bool `json:"enable_caller" toml:"enable_caller" yaml:"enable_caller" mapstructure:"enable_caller"`
	EnableStacktrace bool `json:"enable_stacktrace" toml:"enable_stacktrace" yaml:"enable_stacktrace" mapstructure:"enable_stacktrace"`
	CallerSkip       int  `json:"caller_skip" toml:"caller_skip" yaml:"caller_skip" mapstructure:"caller_skip"`

	// 编码器配置
	TimeFormat   string `json:"time_format" toml:"time_format" yaml:"time_format" mapstructure:"time_format"`
	TimeKey      string `json:"time_key" toml:"time_key" yaml:"time_key" mapstructure:"time_key"`
	LevelKey     string `json:"level_key" toml:"level_key" yaml:"level_key" mapstructure:"level_key"`
	MessageKey   string `json:"message_key" toml:"message_key" yaml:"message_key" mapstructure:"message_key"`
	CallerKey    string `json:"caller_key" toml:"caller_key" yaml:"caller_key" mapstructure:"caller_key"`
	EncodeCaller string `json:"encode_caller" toml:"encode_caller" yaml:"encode_caller" mapstructure:"encode_caller"`
	EncodeLevel  string `json:"encode_level" toml:"encode_level" yaml:"encode_level" mapstructure:"encode_level"`
}

// ConfigError 配置错误.
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("logger config error [%s]: %s", e.Field, e.Message)
}

// Validate 验证配置.
func (c *Config) Validate() error {
	if c == nil {
		return &ConfigError{Field: "config", Message: "config cannot be nil"}
	}

	// 大小写标准化
	c.Level = strings.ToLower(c.Level)
	c.Format = strings.ToLower(c.Format)
	c.Output = strings.ToLower(c.Output)

	if c.Level != "" && !isValidLevel(c.Level) {
		return &ConfigError{Field: "level", Message: "invalid log level: " + c.Level}
	}

	if c.Format != "" && !isValidFormat(c.Format) {
		return &ConfigError{Field: "format", Message: "invalid format: " + c.Format}
	}

	if c.Output != "" && !isValidOutput(c.Output) {
		return &ConfigError{Field: "output", Message: "invalid output: " + c.Output}
	}

	if c.needsFileOutput() && c.LogDir == "" {
		return &ConfigError{Field: "log_dir", Message: "log_dir is required when output is file or both"}
	}

	return nil
}

// ApplyDefaults 应用默认值.
func (c *Config) ApplyDefaults() {
	if c.Type == "" {
		c.Type = TypeZap
	}
	if c.Level == "" {
		c.Level = LevelInfo
	}
	if c.Format == "" {
		c.Format = FormatConsole
	}
	if c.Output == "" {
		c.Output = OutputConsole
	}
	if c.ServiceName == "" {
		c.ServiceName = "service"
	}
	if c.TimeKey == "" {
		c.TimeKey = "timestamp"
	}
	if c.LevelKey == "" {
		c.LevelKey = "level"
	}
	if c.MessageKey == "" {
		c.MessageKey = "msg"
	}
	if c.CallerKey == "" {
		c.CallerKey = "caller"
	}
	if c.TimeFormat == "" {
		c.TimeFormat = TimeFormatDateTime
	}
	if c.EncodeLevel == "" {
		c.EncodeLevel = EncodeLevelCapital
	}
	if c.EncodeCaller == "" {
		c.EncodeCaller = EncodeCallerShort
	}
	if c.RotationTime == "" {
		c.RotationTime = RotationDaily
	}
	if c.MaxAge <= 0 {
		c.MaxAge = 7
	}
}

// needsFileOutput 检查是否需要文件输出.
func (c *Config) needsFileOutput() bool {
	output := strings.ToLower(c.Output)
	return output == OutputFile || output == OutputBoth
}

// shouldOutputToConsole 检查是否应该输出到控制台.
func (c *Config) shouldOutputToConsole() bool {
	if c.ConsoleEnabled {
		return true
	}
	output := strings.ToLower(c.Output)
	return output == OutputConsole || output == OutputBoth
}

// isValidLevel 检查日志级别是否有效.
func isValidLevel(level string) bool {
	switch strings.ToLower(level) {
	case LevelDebug, LevelInfo, LevelWarn, "warning", LevelError, LevelFatal, LevelPanic:
		return true
	}
	return false
}

// isValidFormat 检查格式是否有效.
func isValidFormat(format string) bool {
	switch strings.ToLower(format) {
	case FormatJSON, FormatConsole:
		return true
	}
	return false
}

// isValidOutput 检查输出是否有效.
func isValidOutput(output string) bool {
	switch strings.ToLower(output) {
	case OutputConsole, OutputFile, OutputBoth:
		return true
	}
	return false
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	config := &Config{}
	config.ApplyDefaults()
	return config
}

// NewDevConfig 返回开发环境配置.
func NewDevConfig() *Config {
	return &Config{
		Type:         TypeZap,
		Level:        LevelDebug,
		Format:       FormatConsole,
		Output:       OutputConsole,
		EnableCaller: true,
		EncodeLevel:  EncodeLevelCapitalColor,
	}
}

// NewProdConfig 返回生产环境配置.
func NewProdConfig(serviceName, logDir string) *Config {
	return &Config{
		Type:             TypeZap,
		ServiceName:      serviceName,
		Level:            LevelInfo,
		Format:           FormatJSON,
		Output:           OutputBoth,
		LogDir:           logDir,
		RotationEnabled:  true,
		RotationTime:     RotationDaily,
		MaxAge:           30,
		Compress:         true,
		EnableCaller:     true,
		EnableStacktrace: true,
	}
}
