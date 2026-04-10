package logger

import (
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// zapLogger zap 日志实现.
type zapLogger struct {
	logger  *zap.Logger
	sugar   *zap.SugaredLogger
	writers []RotateWriter
}

// newZapLogger 创建 zap logger.
func newZapLogger(config *Config) (Logger, error) {
	level := parseLevel(config.Level)
	encoder := buildEncoder(config)
	options := buildOptions(config)

	var writers []RotateWriter

	if config.LevelSeparate && config.needsFileOutput() {
		return createLevelSeparateLogger(config, level, encoder, options)
	}

	cores, levelWriters, err := buildCores(config, level, encoder)
	if err != nil {
		return nil, err
	}
	writers = append(writers, levelWriters...)

	var core zapcore.Core
	if len(cores) == 1 {
		core = cores[0]
	} else {
		core = zapcore.NewTee(cores...)
	}

	zapLog := zap.New(core, options...)

	return &zapLogger{
		logger:  zapLog,
		sugar:   zapLog.Sugar(),
		writers: writers,
	}, nil
}

// buildOptions 构建 zap 选项.
func buildOptions(config *Config) []zap.Option {
	var options []zap.Option

	// 用户显式配置优先
	enableCaller := config.EnableCaller

	if enableCaller {
		options = append(options, zap.AddCaller())
		if config.CallerSkip > 0 {
			options = append(options, zap.AddCallerSkip(config.CallerSkip))
		}
	}

	if config.EnableStacktrace {
		options = append(options, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	return options
}

// buildCores 构建日志核心.
func buildCores(config *Config, level zapcore.Level, encoder zapcore.Encoder) ([]zapcore.Core, []RotateWriter, error) {
	var cores []zapcore.Core
	var writers []RotateWriter

	// 文件输出
	if config.needsFileOutput() {
		fileWriter, err := createFileWriter(config, config.ServiceName)
		if err != nil {
			return nil, nil, err
		}
		writers = append(writers, fileWriter)
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(fileWriter), level))
	}

	// 控制台输出
	if config.shouldOutputToConsole() {
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), level))
	}

	if len(cores) == 0 {
		return nil, nil, &ConfigError{Field: "output", Message: "no valid output configured"}
	}

	return cores, writers, nil
}

// createFileWriter 创建文件写入器.
func createFileWriter(config *Config, prefix string) (RotateWriter, error) {
	if config.RotationEnabled {
		return NewRotateWriter(
			config.LogDir,
			prefix,
			WithMaxAge(config.MaxAge),
			WithCompress(config.Compress),
			WithRotationMode(config.RotationTime),
		), nil
	}

	// 静态文件
	dir := filepath.Join(config.LogDir, prefix)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, &ConfigError{Field: "log_dir", Message: "failed to create log directory: " + err.Error()}
	}

	logFile := filepath.Join(dir, prefix+".log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, &ConfigError{Field: "log_dir", Message: "failed to open log file: " + err.Error()}
	}

	return newSyncWriter(file), nil
}

// createLevelSeparateLogger 创建按级别分离的 logger.
func createLevelSeparateLogger(config *Config, minLevel zapcore.Level, encoder zapcore.Encoder, options []zap.Option) (Logger, error) {
	levelConfigs := []struct {
		name  string
		level zapcore.Level
	}{
		{LevelDebug, zapcore.DebugLevel},
		{LevelInfo, zapcore.InfoLevel},
		{LevelWarn, zapcore.WarnLevel},
		{LevelError, zapcore.ErrorLevel},
		{LevelPanic, zapcore.PanicLevel},
		{LevelFatal, zapcore.FatalLevel},
	}

	var cores []zapcore.Core
	var writers []RotateWriter

	for _, lc := range levelConfigs {
		if lc.level < minLevel {
			continue
		}

		var levelWriters []zapcore.WriteSyncer

		// 文件输出
		fileWriter, err := createFileWriter(config, lc.name)
		if err != nil {
			// 清理已创建的 writers
			for _, w := range writers {
				w.Close()
			}
			return nil, err
		}
		writers = append(writers, fileWriter)
		levelWriters = append(levelWriters, zapcore.AddSync(fileWriter))

		// 控制台输出
		if config.shouldOutputToConsole() {
			levelWriters = append(levelWriters, zapcore.AddSync(os.Stdout))
		}

		var writeSyncer zapcore.WriteSyncer
		if len(levelWriters) == 1 {
			writeSyncer = levelWriters[0]
		} else {
			writeSyncer = zapcore.NewMultiWriteSyncer(levelWriters...)
		}

		// 仅记录当前级别
		levelEnabler := zap.LevelEnablerFunc(func(target zapcore.Level) func(zapcore.Level) bool {
			return func(lvl zapcore.Level) bool {
				return lvl == target
			}
		}(lc.level))

		cores = append(cores, zapcore.NewCore(encoder, writeSyncer, levelEnabler))
	}

	if len(cores) == 0 {
		return nil, &ConfigError{Field: "level", Message: "no valid log level configured"}
	}

	zapLog := zap.New(zapcore.NewTee(cores...), options...)

	return &zapLogger{
		logger:  zapLog,
		sugar:   zapLog.Sugar(),
		writers: writers,
	}, nil
}

// 基础日志方法实现

func (z *zapLogger) Debug(args ...any) {
	z.sugar.Debug(args...)
}

func (z *zapLogger) Debugf(format string, args ...any) {
	z.sugar.Debugf(format, args...)
}

func (z *zapLogger) Info(args ...any) {
	z.sugar.Info(args...)
}

func (z *zapLogger) Infof(format string, args ...any) {
	z.sugar.Infof(format, args...)
}

func (z *zapLogger) Warn(args ...any) {
	z.sugar.Warn(args...)
}

func (z *zapLogger) Warnf(format string, args ...any) {
	z.sugar.Warnf(format, args...)
}

func (z *zapLogger) Error(args ...any) {
	z.sugar.Error(args...)
}

func (z *zapLogger) Errorf(format string, args ...any) {
	z.sugar.Errorf(format, args...)
}

func (z *zapLogger) Fatal(args ...any) {
	z.sugar.Fatal(args...)
}

func (z *zapLogger) Fatalf(format string, args ...any) {
	z.sugar.Fatalf(format, args...)
}

func (z *zapLogger) Panic(args ...any) {
	z.sugar.Panic(args...)
}

func (z *zapLogger) Panicf(format string, args ...any) {
	z.sugar.Panicf(format, args...)
}

// With 返回带有附加字段的 logger.
func (z *zapLogger) With(fields ...Field) Logger {
	zapFields := make([]zap.Field, len(fields))
	for i, f := range fields {
		zapFields[i] = toZapField(f)
	}

	newLogger := z.logger.With(zapFields...)
	return &zapLogger{
		logger:  newLogger,
		sugar:   newLogger.Sugar(),
		writers: z.writers,
	}
}

// toZapField 将 Field 转换为 zap.Field.
// 对于复杂类型使用 Reflect，确保走 AddReflected 路径以正确格式化输出.
func toZapField(f Field) zap.Field {
	switch v := f.Value.(type) {
	case string:
		return zap.String(f.Key, v)
	case int:
		return zap.Int(f.Key, v)
	case int64:
		return zap.Int64(f.Key, v)
	case int32:
		return zap.Int32(f.Key, v)
	case int16:
		return zap.Int16(f.Key, v)
	case int8:
		return zap.Int8(f.Key, v)
	case uint:
		return zap.Uint(f.Key, v)
	case uint64:
		return zap.Uint64(f.Key, v)
	case uint32:
		return zap.Uint32(f.Key, v)
	case uint16:
		return zap.Uint16(f.Key, v)
	case uint8:
		return zap.Uint8(f.Key, v)
	case float64:
		return zap.Float64(f.Key, v)
	case float32:
		return zap.Float32(f.Key, v)
	case bool:
		return zap.Bool(f.Key, v)
	case time.Time:
		return zap.Time(f.Key, v)
	case time.Duration:
		return zap.Duration(f.Key, v)
	case error:
		return zap.NamedError(f.Key, v)
	default:
		// 对于 slice、map、struct 等复杂类型，使用 Reflect 确保走 AddReflected 路径
		return zap.Reflect(f.Key, v)
	}
}

// Sync 同步日志缓冲区.
func (z *zapLogger) Sync() error {
	return z.logger.Sync()
}

// Close 关闭 logger 并释放资源.
func (z *zapLogger) Close() error {
	var syncErr error
	if err := z.logger.Sync(); err != nil {
		// stdout/stderr 的 sync 错误可忽略
		// https://github.com/uber-go/zap/issues/328
		// 文件写入的 sync 错误需要保留
		syncErr = err
	}

	// 关闭所有写入器
	for _, w := range z.writers {
		if err := w.Close(); err != nil {
			return err
		}
	}

	// 仅在存在文件写入器时返回 sync 错误（stdout/stderr 的忽略）
	if syncErr != nil && len(z.writers) > 0 {
		return syncErr
	}

	return nil
}

// 便捷字段构造函数

// String 创建字符串字段.
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int 创建整数字段.
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 创建 int64 字段.
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Int32 创建 int32 字段.
func Int32(key string, value int32) Field {
	return Field{Key: key, Value: value}
}

// Int16 创建 int16 字段.
func Int16(key string, value int16) Field {
	return Field{Key: key, Value: value}
}

// Int8 创建 int8 字段.
func Int8(key string, value int8) Field {
	return Field{Key: key, Value: value}
}

// Uint 创建 uint 字段.
func Uint(key string, value uint) Field {
	return Field{Key: key, Value: value}
}

// Uint64 创建 uint64 字段.
func Uint64(key string, value uint64) Field {
	return Field{Key: key, Value: value}
}

// Uint32 创建 uint32 字段.
func Uint32(key string, value uint32) Field {
	return Field{Key: key, Value: value}
}

// Uint16 创建 uint16 字段.
func Uint16(key string, value uint16) Field {
	return Field{Key: key, Value: value}
}

// Uint8 创建 uint8 字段.
func Uint8(key string, value uint8) Field {
	return Field{Key: key, Value: value}
}

// Float64 创建 float64 字段.
func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

// Float32 创建 float32 字段.
func Float32(key string, value float32) Field {
	return Field{Key: key, Value: value}
}

// Bool 创建布尔字段.
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Time 创建时间字段.
func Time(key string, value time.Time) Field {
	return Field{Key: key, Value: value}
}

// Duration 创建持续时间字段.
func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

// Err 创建错误字段.
func Err(err error) Field {
	return Field{Key: "error", Value: err}
}

// Any 创建任意类型字段.
func Any(key string, value any) Field {
	return Field{Key: key, Value: value}
}
