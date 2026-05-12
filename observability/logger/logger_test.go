package logger

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

// LoggerTestSuite logger 测试套件.
type LoggerTestSuite struct {
	suite.Suite
	tmpDir string
}

func TestLoggerSuite(t *testing.T) {
	suite.Run(t, new(LoggerTestSuite))
}

func (s *LoggerTestSuite) SetupTest() {
	s.tmpDir = s.T().TempDir()
}

func (s *LoggerTestSuite) TestNewLogger_NilConfig() {
	log, err := NewLogger(nil)
	s.Error(err)
	s.Nil(log)
}

func (s *LoggerTestSuite) TestNewLogger_DefaultConfig() {
	log, err := NewLogger(DefaultConfig())
	s.NoError(err)
	s.NotNil(log)
	defer log.Close()
}

func (s *LoggerTestSuite) TestMustNewLogger_Success() {
	s.NotPanics(func() {
		log := MustNewLogger(DefaultConfig())
		s.NotNil(log)
		log.Close()
	})
}

func (s *LoggerTestSuite) TestMustNewLogger_Panic() {
	s.Panics(func() {
		MustNewLogger(nil)
	})
}

func (s *LoggerTestSuite) TestNewLogger_DevConfig() {
	log, err := NewLogger(NewDevConfig())
	s.NoError(err)
	s.NotNil(log)
	defer log.Close()
}

func (s *LoggerTestSuite) TestNewLogger_InvalidLevel() {
	config := &Config{Level: "invalid"}
	log, err := NewLogger(config)
	s.Error(err)
	s.Nil(log)
}

func (s *LoggerTestSuite) TestNewLogger_InvalidFormat() {
	config := &Config{Format: "invalid"}
	log, err := NewLogger(config)
	s.Error(err)
	s.Nil(log)
}

func (s *LoggerTestSuite) TestNewLogger_InvalidOutput() {
	config := &Config{Output: "invalid"}
	log, err := NewLogger(config)
	s.Error(err)
	s.Nil(log)
}

func (s *LoggerTestSuite) TestNewLogger_FileOutputWithoutLogDir() {
	config := &Config{Output: OutputFile}
	log, err := NewLogger(config)
	s.Error(err)
	s.Nil(log)
}

func (s *LoggerTestSuite) TestNewLogger_UnsupportedType() {
	config := &Config{
		Type:   "unsupported",
		Output: OutputConsole,
	}
	log, err := NewLogger(config)
	s.Error(err)
	s.Nil(log)
}

func (s *LoggerTestSuite) TestNewLogger_WithFileOutput() {
	config := &Config{
		Level:  LevelDebug,
		Format: FormatJSON,
		Output: OutputFile,
		LogDir: s.tmpDir,
		Type:   TypeZap,
	}

	log, err := NewLogger(config)
	s.Require().NoError(err)
	defer log.Close()

	log.Info("test message")
	log.Sync()

	// 验证文件创建
	serviceDir := filepath.Join(s.tmpDir, "service")
	_, err = os.Stat(serviceDir)
	s.NoError(err, "log directory was not created")
}

func (s *LoggerTestSuite) TestNewLogger_WithBothOutput() {
	config := &Config{
		Level:  LevelInfo,
		Format: FormatConsole,
		Output: OutputBoth,
		LogDir: s.tmpDir,
	}

	log, err := NewLogger(config)
	s.Require().NoError(err)
	defer log.Close()

	log.Info("test both output")
}

func (s *LoggerTestSuite) TestLoggerInterface() {
	log, err := NewLogger(NewDevConfig())
	s.Require().NoError(err)
	defer log.Close()

	// 测试所有日志级别方法不会 panic
	s.NotPanics(func() {
		log.Debug("debug message")
		log.Debugf("debug %s", "formatted")
		log.Info("info message")
		log.Infof("info %s", "formatted")
		log.Warn("warn message")
		log.Warnf("warn %s", "formatted")
		log.Error("error message")
		log.Errorf("error %s", "formatted")
	})

	// 测试 With
	logWithFields := log.With(String("key", "value"), Int("count", 42))
	s.NotNil(logWithFields)
	logWithFields.Info("message with fields")

	// 测试 Sync 不会 panic
	s.NotPanics(func() {
		_ = log.Sync()
	})
}

func (s *LoggerTestSuite) TestLoggerWith() {
	log, err := NewLogger(DefaultConfig())
	s.Require().NoError(err)
	defer log.Close()

	// 链式调用不会 panic
	s.NotPanics(func() {
		log.With(String("service", "test")).
			With(Int("port", 8080)).
			With(Bool("debug", true)).
			Info("chained fields")
	})
}

func (s *LoggerTestSuite) TestFromContextOrUsesContextLoggerFirst() {
	ctxLog := &captureLogger{}
	fallback := &captureLogger{}
	ctx := NewContext(context.Background(), ctxLog)

	FromContextOr(ctx, fallback).Info("来自 context")

	s.Equal([]string{"来自 context"}, ctxLog.messages)
	s.Empty(fallback.messages)
}

func (s *LoggerTestSuite) TestFromContextOrUsesFallbackLogger() {
	fallback := &captureLogger{}

	FromContextOr(context.Background(), fallback).Info("来自 fallback")

	s.Equal([]string{"来自 fallback"}, fallback.messages)
}

func (s *LoggerTestSuite) TestFromContextOrAddsTraceFieldsToFallback() {
	fallback := &captureLogger{}
	ctx := ContextWithTraceID(context.Background(), "trace-1")
	ctx = ContextWithSpanID(ctx, "span-1")

	FromContextOr(ctx, fallback).Info("带链路字段")

	s.Equal([]string{"带链路字段"}, fallback.messages)
	s.Equal("trace-1", fallback.fields["traceId"])
	s.Equal("span-1", fallback.fields["spanId"])
}

type captureLogger struct {
	messages []string
	fields   map[string]any
}

func (l *captureLogger) Debug(args ...any)                 { l.record(args...) }
func (l *captureLogger) Debugf(format string, args ...any) {}
func (l *captureLogger) Info(args ...any)                  { l.record(args...) }
func (l *captureLogger) Infof(format string, args ...any)  {}
func (l *captureLogger) Warn(args ...any)                  { l.record(args...) }
func (l *captureLogger) Warnf(format string, args ...any)  {}
func (l *captureLogger) Error(args ...any)                 { l.record(args...) }
func (l *captureLogger) Errorf(format string, args ...any) {}
func (l *captureLogger) Fatal(args ...any)                 { l.record(args...) }
func (l *captureLogger) Fatalf(format string, args ...any) {}
func (l *captureLogger) Panic(args ...any)                 { l.record(args...) }
func (l *captureLogger) Panicf(format string, args ...any) {}
func (l *captureLogger) With(fields ...Field) Logger {
	if l.fields == nil {
		l.fields = make(map[string]any, len(fields))
	}
	for _, f := range fields {
		l.fields[f.Key] = f.Value
	}
	return l
}
func (l *captureLogger) Sync() error  { return nil }
func (l *captureLogger) Close() error { return nil }

func (l *captureLogger) record(args ...any) {
	if len(args) == 0 {
		return
	}
	if msg, ok := args[0].(string); ok {
		l.messages = append(l.messages, msg)
	}
}

// ConstantsTestSuite 常量测试套件.
type ConstantsTestSuite struct {
	suite.Suite
}

func TestConstantsSuite(t *testing.T) {
	suite.Run(t, new(ConstantsTestSuite))
}

func (s *ConstantsTestSuite) TestTypeConstants() {
	s.Equal("zap", TypeZap)
}

func (s *ConstantsTestSuite) TestLevelConstants() {
	s.Equal("debug", LevelDebug)
	s.Equal("info", LevelInfo)
	s.Equal("warn", LevelWarn)
	s.Equal("error", LevelError)
	s.Equal("fatal", LevelFatal)
	s.Equal("panic", LevelPanic)
}

func (s *ConstantsTestSuite) TestFormatConstants() {
	s.Equal("json", FormatJSON)
	s.Equal("console", FormatConsole)
}

func (s *ConstantsTestSuite) TestOutputConstants() {
	s.Equal("console", OutputConsole)
	s.Equal("file", OutputFile)
	s.Equal("both", OutputBoth)
}
