package logger

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// EncoderTestSuite 编码器测试套件.
type EncoderTestSuite struct {
	suite.Suite
}

func TestEncoderSuite(t *testing.T) {
	suite.Run(t, new(EncoderTestSuite))
}

// TestEncoderBuilder 测试编码器构建器.
func (s *EncoderTestSuite) TestEncoderBuilder() {
	config := &Config{
		Format:       FormatJSON,
		TimeKey:      "ts",
		LevelKey:     "level",
		MessageKey:   "msg",
		CallerKey:    "caller",
		TimeFormat:   TimeFormatDateTime,
		EncodeLevel:  EncodeLevelCapital,
		EncodeCaller: EncodeCallerShort,
	}

	builder := NewEncoderBuilder(config)
	s.NotNil(builder)

	encoder := builder.Build()
	s.NotNil(encoder)
}

// TestEncoderBuilder_JSON 测试 JSON 编码器构建.
func (s *EncoderTestSuite) TestEncoderBuilder_JSON() {
	config := &Config{
		Format:     FormatJSON,
		TimeKey:    "timestamp",
		LevelKey:   "level",
		MessageKey: "message",
	}

	encoder := NewEncoderBuilder(config).Build()
	s.NotNil(encoder)
}

// TestEncoderBuilder_Console 测试 Console 编码器构建.
func (s *EncoderTestSuite) TestEncoderBuilder_Console() {
	config := &Config{
		Format:     FormatConsole,
		TimeKey:    "timestamp",
		LevelKey:   "level",
		MessageKey: "message",
	}

	encoder := NewEncoderBuilder(config).Build()
	s.NotNil(encoder)
}

// TestTimeEncoders 测试各种时间编码器.
func (s *EncoderTestSuite) TestTimeEncoders() {
	testCases := []string{
		TimeFormatISO8601,
		TimeFormatRFC3339,
		TimeFormatRFC3339Nano,
		TimeFormatEpoch,
		TimeFormatEpochMillis,
		TimeFormatEpochNanos,
		TimeFormatDateTime,
		"2006-01-02", // 自定义格式
	}

	for _, format := range testCases {
		config := &Config{TimeFormat: format}
		builder := NewEncoderBuilder(config)
		encoder := builder.getTimeEncoder()
		s.NotNil(encoder, "format: %s", format)
	}
}

// TestLevelEncoders 测试各种级别编码器.
func (s *EncoderTestSuite) TestLevelEncoders() {
	testCases := []string{
		EncodeLevelCapital,
		EncodeLevelCapitalColor,
		EncodeLevelLower,
		EncodeLevelLowerColor,
		"unknown", // 默认为 capital
	}

	for _, encode := range testCases {
		config := &Config{EncodeLevel: encode}
		builder := NewEncoderBuilder(config)
		encoder := builder.getLevelEncoder()
		s.NotNil(encoder, "encode: %s", encode)
	}
}

// TestCallerEncoders 测试各种调用者编码器.
func (s *EncoderTestSuite) TestCallerEncoders() {
	testCases := []string{
		EncodeCallerShort,
		EncodeCallerFull,
		"unknown", // 默认为 short
	}

	for _, encode := range testCases {
		config := &Config{EncodeCaller: encode}
		builder := NewEncoderBuilder(config)
		encoder := builder.getCallerEncoder()
		s.NotNil(encoder, "encode: %s", encode)
	}
}

// TestParseLevel 测试日志级别解析.
func (s *EncoderTestSuite) TestParseLevel() {
	testCases := []struct {
		level string
		want  zapcore.Level
	}{
		{LevelDebug, zapcore.DebugLevel},
		{"DEBUG", zapcore.DebugLevel},
		{LevelInfo, zapcore.InfoLevel},
		{"INFO", zapcore.InfoLevel},
		{LevelWarn, zapcore.WarnLevel},
		{"warning", zapcore.WarnLevel},
		{"WARNING", zapcore.WarnLevel},
		{LevelError, zapcore.ErrorLevel},
		{"ERROR", zapcore.ErrorLevel},
		{LevelFatal, zapcore.FatalLevel},
		{"FATAL", zapcore.FatalLevel},
		{LevelPanic, zapcore.PanicLevel},
		{"PANIC", zapcore.PanicLevel},
		{"unknown", zapcore.InfoLevel}, // 默认为 info
		{"", zapcore.InfoLevel},        // 默认为 info
	}

	for _, tc := range testCases {
		got := parseLevel(tc.level)
		s.Equal(tc.want, got, "level: %s", tc.level)
	}
}

// TestDatetimeEncoder 测试自定义日期时间编码器.
func (s *EncoderTestSuite) TestDatetimeEncoder() {
	testTime := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)
	enc := &mockArrayEncoder{}

	datetimeEncoder(testTime, enc)

	s.Equal("2024-01-15 10:30:45", enc.value)
}

// ConsoleEncoderTestSuite console 编码器测试套件.
type ConsoleEncoderTestSuite struct {
	suite.Suite
}

func TestConsoleEncoderSuite(t *testing.T) {
	suite.Run(t, new(ConsoleEncoderTestSuite))
}

// TestConsoleEncoder_WithFields 测试带字段的输出.
func (s *ConsoleEncoderTestSuite) TestConsoleEncoder_WithFields() {
	buf := &bytes.Buffer{}

	config := zapcore.EncoderConfig{
		TimeKey:          "ts",
		LevelKey:         "level",
		MessageKey:       "msg",
		CallerKey:        "caller",
		EncodeTime:       zapcore.ISO8601TimeEncoder,
		EncodeLevel:      zapcore.CapitalLevelEncoder,
		EncodeCaller:     zapcore.ShortCallerEncoder,
		ConsoleSeparator: "\t",
	}

	encoder := newConsoleEncoder(config)
	core := zapcore.NewCore(encoder, zapcore.AddSync(buf), zapcore.DebugLevel)
	logger := zap.New(core)

	// 使用结构化字段
	logger.Info("test message",
		zap.String("traceId", "abc123"),
		zap.String("spanId", "def456"),
		zap.Duration("elapsed", 100*time.Millisecond),
		zap.Int64("rows", 10),
	)

	output := buf.String()

	// 验证输出不包含 JSON 花括号
	s.NotContains(output, "{")
	s.NotContains(output, "}")

	// 验证包含 [key:value] 格式的字段
	s.Contains(output, "[traceId:abc123]")
	s.Contains(output, "[spanId:def456]")
	s.Contains(output, "[elapsed:100ms]")
	s.Contains(output, "[rows:10]")
	s.Contains(output, "test message")
}

// TestConsoleEncoder_NoFields 测试无字段的输出.
func (s *ConsoleEncoderTestSuite) TestConsoleEncoder_NoFields() {
	buf := &bytes.Buffer{}

	config := zapcore.EncoderConfig{
		TimeKey:          "ts",
		LevelKey:         "level",
		MessageKey:       "msg",
		EncodeTime:       zapcore.ISO8601TimeEncoder,
		EncodeLevel:      zapcore.CapitalLevelEncoder,
		ConsoleSeparator: "\t",
	}

	encoder := newConsoleEncoder(config)
	core := zapcore.NewCore(encoder, zapcore.AddSync(buf), zapcore.DebugLevel)
	logger := zap.New(core)

	logger.Info("simple message")

	output := buf.String()
	s.Contains(output, "simple message")
	s.Contains(output, "INFO")
}

// TestConsoleEncoder_Component 测试组件字段渲染为消息前缀.
func (s *ConsoleEncoderTestSuite) TestConsoleEncoder_Component() {
	buf := &bytes.Buffer{}

	config := zapcore.EncoderConfig{
		MessageKey:       "msg",
		ConsoleSeparator: "\t",
	}

	encoder := newConsoleEncoder(config)
	core := zapcore.NewCore(encoder, zapcore.AddSync(buf), zapcore.DebugLevel)
	log := zap.New(core).With(zap.String(ComponentKey, "Auth"))

	log.Warn("主体已过期", zap.String("principal_id", "user-1"))

	output := buf.String()
	s.Contains(output, "[Auth] 主体已过期")
	s.Contains(output, "[principal_id:user-1]")
	s.NotContains(output, "[component:Auth]")
}

// TestConsoleEncoder_Clone 测试克隆.
func (s *ConsoleEncoderTestSuite) TestConsoleEncoder_Clone() {
	config := zapcore.EncoderConfig{
		TimeKey:          "ts",
		LevelKey:         "level",
		MessageKey:       "msg",
		EncodeTime:       zapcore.ISO8601TimeEncoder,
		EncodeLevel:      zapcore.CapitalLevelEncoder,
		ConsoleSeparator: "\t",
	}

	encoder := newConsoleEncoder(config)
	cloned := encoder.Clone()

	s.NotNil(cloned)
	s.IsType(&consoleEncoder{}, cloned)
}

// TestConsoleEncoder_FieldTypes 测试各种字段类型.
func (s *ConsoleEncoderTestSuite) TestConsoleEncoder_FieldTypes() {
	buf := &bytes.Buffer{}

	config := zapcore.EncoderConfig{
		TimeKey:          "",
		LevelKey:         "",
		MessageKey:       "msg",
		ConsoleSeparator: "\t",
	}

	encoder := newConsoleEncoder(config)
	core := zapcore.NewCore(encoder, zapcore.AddSync(buf), zapcore.DebugLevel)
	logger := zap.New(core)

	// 测试各种字段类型
	logger.Info("types test",
		zap.String("str", "hello"),
		zap.Int("int", 42),
		zap.Int64("int64", 9999999999),
		zap.Bool("bool_true", true),
		zap.Bool("bool_false", false),
		zap.Duration("duration", 5*time.Second),
		zap.Float64("float", 3.14),
	)

	output := buf.String()

	s.Contains(output, "[str:hello]")
	s.Contains(output, "[int:42]")
	s.Contains(output, "[int64:9999999999]")
	s.Contains(output, "[bool_true:true]")
	s.Contains(output, "[bool_false:false]")
	s.Contains(output, "[duration:5s]")
	s.Contains(output, "[float:3.14]")
}

// TestConsoleEncoder_WithError 测试错误字段.
func (s *ConsoleEncoderTestSuite) TestConsoleEncoder_WithError() {
	buf := &bytes.Buffer{}

	config := zapcore.EncoderConfig{
		TimeKey:          "",
		LevelKey:         "",
		MessageKey:       "msg",
		ConsoleSeparator: "\t",
	}

	encoder := newConsoleEncoder(config)
	core := zapcore.NewCore(encoder, zapcore.AddSync(buf), zapcore.DebugLevel)
	logger := zap.New(core)

	testErr := &testError{msg: "test error message"}
	logger.Error("error occurred", zap.Error(testErr))

	output := buf.String()
	s.Contains(output, "[error:test error message]")
}

// TestConsoleEncoder_UintTypes 测试无符号整数类型.
func (s *ConsoleEncoderTestSuite) TestConsoleEncoder_UintTypes() {
	buf := &bytes.Buffer{}

	config := zapcore.EncoderConfig{
		TimeKey:          "",
		LevelKey:         "",
		MessageKey:       "msg",
		ConsoleSeparator: "\t",
	}

	encoder := newConsoleEncoder(config)
	core := zapcore.NewCore(encoder, zapcore.AddSync(buf), zapcore.DebugLevel)
	logger := zap.New(core)

	logger.Info("uint test",
		zap.Uint("uint", 100),
		zap.Uint64("uint64", 18446744073709551615),
		zap.Uint32("uint32", 4294967295),
	)

	output := buf.String()
	s.Contains(output, "[uint:100]")
	s.Contains(output, "[uint64:18446744073709551615]")
	s.Contains(output, "[uint32:4294967295]")
}

// TestConsoleEncoder_WithMethod 测试 With() 方法添加的字段.
func (s *ConsoleEncoderTestSuite) TestConsoleEncoder_WithMethod() {
	buf := &bytes.Buffer{}

	config := zapcore.EncoderConfig{
		TimeKey:          "",
		LevelKey:         "level",
		MessageKey:       "msg",
		EncodeLevel:      zapcore.CapitalLevelEncoder,
		ConsoleSeparator: "\t",
	}

	encoder := newConsoleEncoder(config)
	core := zapcore.NewCore(encoder, zapcore.AddSync(buf), zapcore.DebugLevel)
	baseLogger := zap.New(core)

	// 使用 With() 添加字段
	logger := baseLogger.With(
		zap.String("traceId", "abc123"),
		zap.String("spanId", "def456"),
	)

	// 记录日志，同时传入额外字段
	logger.Info("test message",
		zap.Duration("elapsed", 100*time.Millisecond),
		zap.Int64("rows", 10),
	)

	output := buf.String()

	// 验证输出不包含 JSON 花括号
	s.NotContains(output, "{")
	s.NotContains(output, "}")

	// 验证 With() 添加的字段
	s.Contains(output, "[traceId:abc123]")
	s.Contains(output, "[spanId:def456]")

	// 验证本次调用传入的字段
	s.Contains(output, "[elapsed:100ms]")
	s.Contains(output, "[rows:10]")
	s.Contains(output, "test message")
}

// mockArrayEncoder 用于测试的 mock encoder.
type mockArrayEncoder struct {
	value string
}

func (m *mockArrayEncoder) AppendBool(_ bool)                          {}
func (m *mockArrayEncoder) AppendByteString(_ []byte)                  {}
func (m *mockArrayEncoder) AppendComplex128(_ complex128)              {}
func (m *mockArrayEncoder) AppendComplex64(_ complex64)                {}
func (m *mockArrayEncoder) AppendFloat64(_ float64)                    {}
func (m *mockArrayEncoder) AppendFloat32(_ float32)                    {}
func (m *mockArrayEncoder) AppendInt(_ int)                            {}
func (m *mockArrayEncoder) AppendInt64(_ int64)                        {}
func (m *mockArrayEncoder) AppendInt32(_ int32)                        {}
func (m *mockArrayEncoder) AppendInt16(_ int16)                        {}
func (m *mockArrayEncoder) AppendInt8(_ int8)                          {}
func (m *mockArrayEncoder) AppendString(v string)                      { m.value = v }
func (m *mockArrayEncoder) AppendUint(_ uint)                          {}
func (m *mockArrayEncoder) AppendUint64(_ uint64)                      {}
func (m *mockArrayEncoder) AppendUint32(_ uint32)                      {}
func (m *mockArrayEncoder) AppendUint16(_ uint16)                      {}
func (m *mockArrayEncoder) AppendUint8(_ uint8)                        {}
func (m *mockArrayEncoder) AppendUintptr(_ uintptr)                    {}
func (m *mockArrayEncoder) AppendDuration(_ time.Duration)             {}
func (m *mockArrayEncoder) AppendTime(_ time.Time)                     {}
func (m *mockArrayEncoder) AppendArray(_ zapcore.ArrayMarshaler) error { return nil }
func (m *mockArrayEncoder) AppendObject(_ zapcore.ObjectMarshaler) error {
	return nil
}
func (m *mockArrayEncoder) AppendReflected(_ any) error { return nil }

// testError 用于测试的错误类型.
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
