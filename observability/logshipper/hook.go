package logshipper

import (
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// ────────────────────────────────────────────────────────────────────────────
// ZapHook：适用于能直接访问 *zap.Logger 的场景
// ────────────────────────────────────────────────────────────────────────────

// shipperCore 是一个 zapcore.Core 实现，将日志条目投递到 Shipper.
// 它本身不输出日志，仅做旁路投递.
type shipperCore struct {
	shipper *Shipper
	fields  []zapcore.Field
}

// Enabled 对所有级别返回 true，由 Shipper 层面决定是否丢弃.
func (c *shipperCore) Enabled(_ zapcore.Level) bool { return true }

// With 返回带有附加字段的新 Core.
func (c *shipperCore) With(fields []zapcore.Field) zapcore.Core {
	copied := make([]zapcore.Field, len(c.fields)+len(fields))
	copy(copied, c.fields)
	copy(copied[len(c.fields):], fields)
	return &shipperCore{shipper: c.shipper, fields: copied}
}

// Check 将当前条目标记为待写入.
func (c *shipperCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	return ce.AddCore(entry, c)
}

// Write 将 zapcore.Entry 转换为 logshipper.Entry，并调用 Shipper.Ship.
func (c *shipperCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	logEntry := Entry{
		Timestamp:  entry.Time,
		Level:      entry.Level.String(),
		Message:    entry.Message,
		Logger:     entry.LoggerName,
		Caller:     entry.Caller.String(),
		StackTrace: entry.Stack,
		Fields:     zapFieldsToMap(append(c.fields, fields...)),
	}
	c.shipper.Ship(logEntry)
	return nil
}

// Sync 无操作，Shipper 自己管理刷新.
func (c *shipperCore) Sync() error { return nil }

// ZapHook 返回一个 zapcore.Core，将日志同时投递到 Shipper.
// 用法示例:
//
//	hook := ZapHook(shipper)
//	logger := zap.New(zapcore.NewTee(originalCore, hook))
func ZapHook(shipper *Shipper) zapcore.Core {
	return &shipperCore{shipper: shipper}
}

// AttachToLogger 将 Shipper 附加到已有的 *zap.Logger通过 zap.WrapCore 组合 Core.
// 返回新的 logger，原 logger 不变.
func AttachToLogger(log *zap.Logger, shipper *Shipper) *zap.Logger {
	hook := ZapHook(shipper)
	return log.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewTee(core, hook)
	}))
}

// zapFieldsToMap 将 zapcore.Field 列表转换为 map[string]any.
func zapFieldsToMap(fields []zapcore.Field) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fields {
		f.AddTo(enc)
	}
	return enc.Fields
}

// ────────────────────────────────────────────────────────────────────────────
// NewLoggerHook：适用于只有 logger.Logger 接口的场景
// ────────────────────────────────────────────────────────────────────────────

// loggerHook 实现 logger.Logger，在委托给 inner 的同时将日志投递到 Shipper.
type loggerHook struct {
	inner    logger.Logger
	shipper  *Shipper
	minLevel int // 对应 zapcore.Level 的整数值
	fields   map[string]any
}

// parseMinLevel 将级别字符串解析为 zapcore.Level 整数值.
func parseMinLevel(level string) int {
	switch strings.ToLower(level) {
	case "debug":
		return int(zapcore.DebugLevel)
	case "info":
		return int(zapcore.InfoLevel)
	case "warn", "warning":
		return int(zapcore.WarnLevel)
	case "error":
		return int(zapcore.ErrorLevel)
	case "fatal":
		return int(zapcore.FatalLevel)
	case "panic":
		return int(zapcore.PanicLevel)
	default:
		return int(zapcore.InfoLevel)
	}
}

// NewLoggerHook 返回一个 logger.Logger 包装器，将日志同时投递到 Shipper.
// minLevel 控制最低投递级别例如 "info" 则 debug 日志不投递.
func NewLoggerHook(inner logger.Logger, shipper *Shipper, minLevel string) logger.Logger {
	return &loggerHook{
		inner:    inner,
		shipper:  shipper,
		minLevel: parseMinLevel(minLevel),
		fields:   nil,
	}
}

// ship 构造 Entry 并调用 Shipper.Ship，仅在满足最低级别时执行.
func (h *loggerHook) ship(level zapcore.Level, msg string) {
	if int(level) < h.minLevel {
		return
	}

	var fields map[string]any
	if len(h.fields) > 0 {
		fields = make(map[string]any, len(h.fields))
		for k, v := range h.fields {
			fields[k] = v
		}
	}

	h.shipper.Ship(Entry{
		Timestamp: time.Now(),
		Level:     level.String(),
		Message:   msg,
		Fields:    fields,
	})
}

// argsToString 将可变参数合并为字符串.
func argsToString(args []any) string {
	if len(args) == 0 {
		return ""
	}
	if len(args) == 1 {
		return fmt.Sprint(args[0])
	}
	return fmt.Sprint(args...)
}

func (h *loggerHook) Debug(args ...any) {
	h.inner.Debug(args...)
	h.ship(zapcore.DebugLevel, argsToString(args))
}

func (h *loggerHook) Debugf(format string, args ...any) {
	h.inner.Debugf(format, args...)
	h.ship(zapcore.DebugLevel, fmt.Sprintf(format, args...))
}

func (h *loggerHook) Info(args ...any) {
	h.inner.Info(args...)
	h.ship(zapcore.InfoLevel, argsToString(args))
}

func (h *loggerHook) Infof(format string, args ...any) {
	h.inner.Infof(format, args...)
	h.ship(zapcore.InfoLevel, fmt.Sprintf(format, args...))
}

func (h *loggerHook) Warn(args ...any) {
	h.inner.Warn(args...)
	h.ship(zapcore.WarnLevel, argsToString(args))
}

func (h *loggerHook) Warnf(format string, args ...any) {
	h.inner.Warnf(format, args...)
	h.ship(zapcore.WarnLevel, fmt.Sprintf(format, args...))
}

func (h *loggerHook) Error(args ...any) {
	h.inner.Error(args...)
	h.ship(zapcore.ErrorLevel, argsToString(args))
}

func (h *loggerHook) Errorf(format string, args ...any) {
	h.inner.Errorf(format, args...)
	h.ship(zapcore.ErrorLevel, fmt.Sprintf(format, args...))
}

func (h *loggerHook) Fatal(args ...any) {
	h.ship(zapcore.FatalLevel, argsToString(args))
	h.inner.Fatal(args...)
}

func (h *loggerHook) Fatalf(format string, args ...any) {
	h.ship(zapcore.FatalLevel, fmt.Sprintf(format, args...))
	h.inner.Fatalf(format, args...)
}

func (h *loggerHook) Panic(args ...any) {
	h.ship(zapcore.PanicLevel, argsToString(args))
	h.inner.Panic(args...)
}

func (h *loggerHook) Panicf(format string, args ...any) {
	h.ship(zapcore.PanicLevel, fmt.Sprintf(format, args...))
	h.inner.Panicf(format, args...)
}

// With 返回带有附加字段的新 loggerHook.
func (h *loggerHook) With(fields ...logger.Field) logger.Logger {
	merged := make(map[string]any, len(h.fields)+len(fields))
	for k, v := range h.fields {
		merged[k] = v
	}
	for _, f := range fields {
		merged[f.Key] = f.Value
	}
	return &loggerHook{
		inner:    h.inner.With(fields...),
		shipper:  h.shipper,
		minLevel: h.minLevel,
		fields:   merged,
	}
}

// Sync 同步日志缓冲区.
func (h *loggerHook) Sync() error {
	return h.inner.Sync()
}

// Close 关闭 inner logger，Shipper 的生命周期由调用者管理.
func (h *loggerHook) Close() error {
	return h.inner.Close()
}
