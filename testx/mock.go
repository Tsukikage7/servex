// Package testx 提供测试辅助工具集.
package testx

import (
	"fmt"
	"testing"

	"github.com/Tsukikage7/servex/observability/logger"
)

// nopLogger 空操作日志记录器，丢弃所有日志输出.
type nopLogger struct{}

func (n *nopLogger) Debug(_ ...any)                       {}
func (n *nopLogger) Debugf(_ string, _ ...any)            {}
func (n *nopLogger) Info(_ ...any)                        {}
func (n *nopLogger) Infof(_ string, _ ...any)             {}
func (n *nopLogger) Warn(_ ...any)                        {}
func (n *nopLogger) Warnf(_ string, _ ...any)             {}
func (n *nopLogger) Error(_ ...any)                       {}
func (n *nopLogger) Errorf(_ string, _ ...any)            {}
func (n *nopLogger) Fatal(_ ...any)                       {}
func (n *nopLogger) Fatalf(_ string, _ ...any)            {}
func (n *nopLogger) Panic(_ ...any)                       {}
func (n *nopLogger) Panicf(_ string, _ ...any)            {}
func (n *nopLogger) With(_ ...logger.Field) logger.Logger { return n }
func (n *nopLogger) Sync() error                          { return nil }
func (n *nopLogger) Close() error                         { return nil }

// NopLogger 返回一个空操作日志记录器，实现了 logger.Logger 的所有方法但不产生任何输出.
func NopLogger() logger.Logger {
	return &nopLogger{}
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
func (l *testLogger) Fatal(args ...any) { l.t.Helper(); l.log("FATAL", args...) }
func (l *testLogger) Fatalf(format string, args ...any) {
	l.t.Helper()
	l.logf("FATAL", format, args...)
}
func (l *testLogger) Panic(args ...any) { l.t.Helper(); l.log("PANIC", args...) }
func (l *testLogger) Panicf(format string, args ...any) {
	l.t.Helper()
	l.logf("PANIC", format, args...)
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
