// Package logger 提供结构化日志记录功能.
package logger

import "context"

const (
	// TypeZap 基于 zap 的日志实现.
	TypeZap = "zap"
)

const (
	// LevelDebug 调试级别.
	LevelDebug = "debug"
	// LevelInfo 信息级别.
	LevelInfo = "info"
	// LevelWarn 警告级别.
	LevelWarn = "warn"
	// LevelError 错误级别.
	LevelError = "error"
	// LevelFatal 致命级别.
	LevelFatal = "fatal"
	// LevelPanic 恐慌级别.
	LevelPanic = "panic"
)

const (
	// FormatJSON JSON 输出格式.
	FormatJSON = "json"
	// FormatConsole 控制台输出格式.
	FormatConsole = "console"
)

const (
	// OutputConsole 输出到控制台.
	OutputConsole = "console"
	// OutputFile 输出到文件.
	OutputFile = "file"
	// OutputBoth 同时输出到控制台和文件.
	OutputBoth = "both"
)

const (
	// RotationDaily 按天轮转.
	RotationDaily = "daily"
	// RotationHourly 按小时轮转.
	RotationHourly = "hourly"
)

const (
	// TimeFormatISO8601 ISO8601 时间格式.
	TimeFormatISO8601 = "iso8601"
	// TimeFormatRFC3339 RFC3339 时间格式.
	TimeFormatRFC3339 = "rfc3339"
	// TimeFormatRFC3339Nano RFC3339 纳秒精度时间格式.
	TimeFormatRFC3339Nano = "rfc3339nano"
	// TimeFormatEpoch Unix 时间戳格式.
	TimeFormatEpoch = "epoch"
	// TimeFormatEpochMillis 毫秒时间戳格式.
	TimeFormatEpochMillis = "epochmillis"
	// TimeFormatEpochNanos 纳秒时间戳格式.
	TimeFormatEpochNanos = "epochnanos"
	// TimeFormatDateTime 日期时间格式.
	TimeFormatDateTime = "datetime"
)

const (
	// EncodeLevelCapital 大写级别编码.
	EncodeLevelCapital = "capital"
	// EncodeLevelCapitalColor 大写彩色级别编码.
	EncodeLevelCapitalColor = "capitalcolor"
	// EncodeLevelLower 小写级别编码.
	EncodeLevelLower = "lower"
	// EncodeLevelLowerColor 小写彩色级别编码.
	EncodeLevelLowerColor = "lowercolor"
)

const (
	// EncodeCallerShort 短路径调用者编码.
	EncodeCallerShort = "short"
	// EncodeCallerFull 完整路径调用者编码.
	EncodeCallerFull = "full"
)

// contextKey context 键类型.
type contextKey string

const (
	// TraceIDKey 用于在 context 中存储 traceId.
	TraceIDKey contextKey = "logger:traceId"
	// SpanIDKey 用于在 context 中存储 spanId.
	SpanIDKey contextKey = "logger:spanId"
	// loggerKey 用于在 context 中存储 Logger 实例.
	loggerKey contextKey = "logger:instance"
)

// Field 表示一个日志字段.
type Field struct {
	Key   string
	Value any
}

// Logger 日志记录器接口.
type Logger interface {
	// 基础日志方法
	Debug(args ...any)
	Debugf(format string, args ...any)
	Info(args ...any)
	Infof(format string, args ...any)
	Warn(args ...any)
	Warnf(format string, args ...any)
	Error(args ...any)
	Errorf(format string, args ...any)
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Panic(args ...any)
	Panicf(format string, args ...any)

	// 结构化日志方法
	With(fields ...Field) Logger

	// 生命周期管理
	Sync() error
	Close() error
}

// NewContext 将 Logger 存入 context.
// 通常在中间件中调用，将带 traceId 等字段的 logger 注入到 context，
// 业务代码通过 FromContext 取出即可自动携带链路信息.
func NewContext(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// FromContext 从 context 中取出 Logger.
// 如果 context 中没有 Logger（中间件未注入），回退到从 context 中提取
// traceId/spanId 构建一个带链路信息的 nop logger.
// 业务代码推荐用法:
//
//	logger.FromContext(ctx).Info("处理请求")
func FromContext(ctx context.Context) Logger {
	if ctx == nil {
		return &nopLogger{}
	}
	if l, ok := ctx.Value(loggerKey).(Logger); ok {
		return l
	}
	return &nopLogger{}
}

// ContextWithTraceID 将 traceId 注入到 context.
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// ContextWithSpanID 将 spanId 注入到 context.
func ContextWithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, SpanIDKey, spanID)
}

// nopLogger 空日志实现，FromContext 在 context 中找不到 logger 时的回退.
type nopLogger struct{}

func (n *nopLogger) Debug(...any)          {}
func (n *nopLogger) Debugf(string, ...any) {}
func (n *nopLogger) Info(...any)           {}
func (n *nopLogger) Infof(string, ...any)  {}
func (n *nopLogger) Warn(...any)           {}
func (n *nopLogger) Warnf(string, ...any)  {}
func (n *nopLogger) Error(...any)          {}
func (n *nopLogger) Errorf(string, ...any) {}
func (n *nopLogger) Fatal(...any)          {}
func (n *nopLogger) Fatalf(string, ...any) {}
func (n *nopLogger) Panic(...any)          {}
func (n *nopLogger) Panicf(string, ...any) {}
func (n *nopLogger) With(...Field) Logger  { return n }
func (n *nopLogger) Sync() error           { return nil }
func (n *nopLogger) Close() error          { return nil }

// NewLogger 创建 logger 实例.
func NewLogger(config *Config) (Logger, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	config.ApplyDefaults()

	switch config.Type {
	case TypeZap, "":
		return newZapLogger(config)
	default:
		return nil, &ConfigError{Field: "type", Message: "unsupported logger type: " + config.Type}
	}
}

// MustNewLogger 创建 logger 实例，失败时 panic.
func MustNewLogger(config *Config) Logger {
	l, err := NewLogger(config)
	if err != nil {
		panic(err)
	}
	return l
}
