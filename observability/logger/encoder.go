package logger

import (
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Encoder 编码器接口.
type Encoder interface {
	zapcore.Encoder
}

// EncoderBuilder 编码器构建器.
type EncoderBuilder struct {
	config *Config
}

// NewEncoderBuilder 创建编码器构建器.
func NewEncoderBuilder(config *Config) *EncoderBuilder {
	return &EncoderBuilder{config: config}
}

// Build 构建编码器.
func (b *EncoderBuilder) Build() zapcore.Encoder {
	if b.isJSON() {
		return b.buildJSONEncoder()
	}
	return b.buildConsoleEncoder()
}

// isJSON 判断是否为 JSON 格式.
func (b *EncoderBuilder) isJSON() bool {
	return strings.EqualFold(b.config.Format, FormatJSON)
}

// buildJSONEncoder 构建 JSON 编码器.
func (b *EncoderBuilder) buildJSONEncoder() zapcore.Encoder {
	return zapcore.NewJSONEncoder(b.buildConfig())
}

// buildConsoleEncoder 构建 Console 编码器.
func (b *EncoderBuilder) buildConsoleEncoder() zapcore.Encoder {
	cfg := b.buildConfig()
	cfg.ConsoleSeparator = "\t"
	cfg.EncodeDuration = zapcore.StringDurationEncoder
	return newConsoleEncoder(cfg)
}

// buildConfig 构建编码器配置.
func (b *EncoderBuilder) buildConfig() zapcore.EncoderConfig {
	cfg := zap.NewProductionEncoderConfig()

	cfg.TimeKey = b.config.TimeKey
	cfg.LevelKey = b.config.LevelKey
	cfg.MessageKey = b.config.MessageKey
	cfg.CallerKey = b.config.CallerKey

	cfg.EncodeTime = b.getTimeEncoder()
	cfg.EncodeLevel = b.getLevelEncoder()
	cfg.EncodeCaller = b.getCallerEncoder()

	return cfg
}

// getTimeEncoder 获取时间编码器.
func (b *EncoderBuilder) getTimeEncoder() zapcore.TimeEncoder {
	switch strings.ToLower(b.config.TimeFormat) {
	case TimeFormatISO8601:
		return zapcore.ISO8601TimeEncoder
	case TimeFormatRFC3339:
		return zapcore.RFC3339TimeEncoder
	case TimeFormatRFC3339Nano:
		return zapcore.RFC3339NanoTimeEncoder
	case TimeFormatEpoch:
		return zapcore.EpochTimeEncoder
	case TimeFormatEpochMillis:
		return zapcore.EpochMillisTimeEncoder
	case TimeFormatEpochNanos:
		return zapcore.EpochNanosTimeEncoder
	case TimeFormatDateTime:
		return datetimeEncoder
	default:
		return zapcore.TimeEncoderOfLayout(b.config.TimeFormat)
	}
}

// getLevelEncoder 获取级别编码器.
func (b *EncoderBuilder) getLevelEncoder() zapcore.LevelEncoder {
	switch strings.ToLower(b.config.EncodeLevel) {
	case EncodeLevelCapital:
		return zapcore.CapitalLevelEncoder
	case EncodeLevelCapitalColor:
		return zapcore.CapitalColorLevelEncoder
	case EncodeLevelLower:
		return zapcore.LowercaseLevelEncoder
	case EncodeLevelLowerColor:
		return zapcore.LowercaseColorLevelEncoder
	default:
		return zapcore.CapitalLevelEncoder
	}
}

// getCallerEncoder 获取调用者编码器.
func (b *EncoderBuilder) getCallerEncoder() zapcore.CallerEncoder {
	switch strings.ToLower(b.config.EncodeCaller) {
	case EncodeCallerShort:
		return zapcore.ShortCallerEncoder
	case EncodeCallerFull:
		return zapcore.FullCallerEncoder
	default:
		return zapcore.ShortCallerEncoder
	}
}

// datetimeEncoder 自定义日期时间编码器.
func datetimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05"))
}

// parseLevel 解析日志级别.
func parseLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case LevelDebug:
		return zapcore.DebugLevel
	case LevelInfo:
		return zapcore.InfoLevel
	case LevelWarn, "warning":
		return zapcore.WarnLevel
	case LevelError:
		return zapcore.ErrorLevel
	case LevelFatal:
		return zapcore.FatalLevel
	case LevelPanic:
		return zapcore.PanicLevel
	default:
		return zapcore.InfoLevel
	}
}
