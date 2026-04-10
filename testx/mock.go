// Package testx 提供测试辅助工具集.
package testx

import (
	"fmt"
	"testing"

	"github.com/Tsukikage7/servex/observability/logger"
)

// NopLogger 返回一个空操作日志记录器.
// 内部委托给 logger.Nop()，避免重复实现.
func NopLogger() logger.Logger {
	return logger.Nop()
}

// testLogger 将日志输出转发到 testing.T 的日志记录器.
type testLogger struct {
	t      *testing.T
	fields []logger.Field
}

func (l *testLogger) log(level string, args ...any) {
	l.t.Helper()
	prefix := l.fieldPrefix()
	l.t.Log(append([]any{"[" + level + "]" + prefix}, args...)...)
}

func (l *testLogger) logf(level, format string, args ...any) {
	l.t.Helper()
	prefix := l.fieldPrefix()
	l.t.Logf("[%s]%s "+format, append([]any{level, prefix}, args...)...)
}

func (l *testLogger) fieldPrefix() string {
	if len(l.fields) == 0 {
		return ""
	}
	s := " "
	for i, f := range l.fields {
		if i > 0 {
			s += " "
		}
		s += fmt.Sprintf("%s=%v", f.Key, f.Value)
	}
	return s
}

func (l *testLogger) Debug(args ...any) { l.t.Helper(); l.log("DEBUG", args...) }
func (l *testLogger) Debugf(format string, args ...any) {
	l.t.Helper()
	l.logf("DEBUG", format, args...)
}
func (l *testLogger) Info(args ...any)                 { l.t.Helper(); l.log("INFO", args...) }
func (l *testLogger) Infof(format string, args ...any) { l.t.Helper(); l.logf("INFO", format, args...) }
func (l *testLogger) Warn(args ...any)                 { l.t.Helper(); l.log("WARN", args...) }
func (l *testLogger) Warnf(format string, args ...any) { l.t.Helper(); l.logf("WARN", format, args...) }
func (l *testLogger) Error(args ...any)                { l.t.Helper(); l.log("ERROR", args...) }
func (l *testLogger) Errorf(format string, args ...any) {
	l.t.Helper()
	l.logf("ERROR", format, args...)
}
func (l *testLogger) Fatal(args ...any) { l.t.Helper(); l.log("FATAL", args...); l.t.FailNow() }
func (l *testLogger) Fatalf(format string, args ...any) {
	l.t.Helper()
	l.logf("FATAL", format, args...)
	l.t.FailNow()
}
func (l *testLogger) Panic(args ...any) { l.t.Helper(); l.log("PANIC", args...); l.t.FailNow() }
func (l *testLogger) Panicf(format string, args ...any) {
	l.t.Helper()
	l.logf("PANIC", format, args...)
	l.t.FailNow()
}

func (l *testLogger) With(fields ...logger.Field) logger.Logger {
	merged := make([]logger.Field, len(l.fields)+len(fields))
	copy(merged, l.fields)
	copy(merged[len(l.fields):], fields)
	return &testLogger{t: l.t, fields: merged}
}

func (l *testLogger) Sync() error  { return nil }
func (l *testLogger) Close() error { return nil }

// TestLogger 返回一个将日志输出到 testing.T 的日志记录器.
func TestLogger(t *testing.T) logger.Logger {
	t.Helper()
	return &testLogger{t: t}
}
