package logger

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

var bufferPool = buffer.NewPool()

// consoleEncoder 自定义 console 编码器.
// 将额外字段以 key=value 格式输出，而非 JSON.
type consoleEncoder struct {
	config    zapcore.EncoderConfig
	fields    *buffer.Buffer // 存储通过 With() 添加的字段
	fieldsNum int            // 字段数量
	component string         // 组件名，控制台输出时渲染为 [Component] 前缀
}

// newConsoleEncoder 创建自定义 console 编码器.
func newConsoleEncoder(config zapcore.EncoderConfig) zapcore.Encoder {
	return &consoleEncoder{
		config:    config,
		fields:    bufferPool.Get(),
		fieldsNum: 0,
	}
}

// Clone 克隆编码器.
func (c *consoleEncoder) Clone() zapcore.Encoder {
	clone := &consoleEncoder{
		config:    c.config,
		fields:    bufferPool.Get(),
		fieldsNum: c.fieldsNum,
		component: c.component,
	}
	// 复制已有字段
	if c.fields.Len() > 0 {
		clone.fields.AppendString(c.fields.String())
	}
	return clone
}

// EncodeEntry 编码日志条目.
func (c *consoleEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	buf := bufferPool.Get()
	component := c.component

	// 编码时间
	if c.config.TimeKey != "" && c.config.EncodeTime != nil {
		c.config.EncodeTime(entry.Time, &primitiveEncoder{buf: buf})
		buf.AppendString(c.config.ConsoleSeparator)
	}

	// 编码级别
	if c.config.LevelKey != "" && c.config.EncodeLevel != nil {
		c.config.EncodeLevel(entry.Level, &primitiveEncoder{buf: buf})
		buf.AppendString(c.config.ConsoleSeparator)
	}

	// 编码调用者
	if entry.Caller.Defined && c.config.CallerKey != "" && c.config.EncodeCaller != nil {
		c.config.EncodeCaller(entry.Caller, &primitiveEncoder{buf: buf})
		buf.AppendString(c.config.ConsoleSeparator)
	}

	// 编码消息
	for _, field := range fields {
		if v, ok := componentFromField(field); ok {
			component = v
			break
		}
	}
	if component != "" {
		buf.AppendByte('[')
		buf.AppendString(component)
		buf.AppendByte(']')
		if entry.Message != "" {
			buf.AppendByte(' ')
		}
	}
	buf.AppendString(entry.Message)

	// 编码通过 With() 添加的字段
	hasWithFields := c.fields.Len() > 0
	callFields := fieldsWithoutComponent(fields)
	hasCallFields := len(callFields) > 0

	if hasWithFields {
		buf.AppendString(c.config.ConsoleSeparator)
		buf.AppendString(c.fields.String())
	}

	// 编码本次调用传入的字段
	if hasCallFields {
		if !hasWithFields {
			buf.AppendString(c.config.ConsoleSeparator)
		}
		written := 0
		for _, field := range callFields {
			if written > 0 || c.fieldsNum > 0 {
				buf.AppendByte(' ')
			}
			c.encodeFieldTo(buf, field)
			written++
		}
	}

	buf.AppendByte('\n')
	return buf, nil
}

// 实现 zapcore.ObjectEncoder 接口，用于处理 With() 添加的字段

// AddString 添加字符串字段.
func (c *consoleEncoder) AddString(key, val string) {
	if key == ComponentKey {
		c.component = val
		return
	}
	c.addSeparator()
	formatField(c.fields, key, val)
}

// AddInt64 添加 int64 字段.
func (c *consoleEncoder) AddInt64(key string, val int64) {
	c.addSeparator()
	formatField(c.fields, key, strconv.FormatInt(val, 10))
}

// AddInt 添加 int 字段.
func (c *consoleEncoder) AddInt(key string, val int) {
	c.AddInt64(key, int64(val))
}

// AddInt32 添加 int32 字段.
func (c *consoleEncoder) AddInt32(key string, val int32) {
	c.AddInt64(key, int64(val))
}

// AddInt16 添加 int16 字段.
func (c *consoleEncoder) AddInt16(key string, val int16) {
	c.AddInt64(key, int64(val))
}

// AddInt8 添加 int8 字段.
func (c *consoleEncoder) AddInt8(key string, val int8) {
	c.AddInt64(key, int64(val))
}

// AddUint64 添加 uint64 字段.
func (c *consoleEncoder) AddUint64(key string, val uint64) {
	c.addSeparator()
	formatField(c.fields, key, strconv.FormatUint(val, 10))
}

// AddUint 添加 uint 字段.
func (c *consoleEncoder) AddUint(key string, val uint) {
	c.AddUint64(key, uint64(val))
}

// AddUint32 添加 uint32 字段.
func (c *consoleEncoder) AddUint32(key string, val uint32) {
	c.AddUint64(key, uint64(val))
}

// AddUint16 添加 uint16 字段.
func (c *consoleEncoder) AddUint16(key string, val uint16) {
	c.AddUint64(key, uint64(val))
}

// AddUint8 添加 uint8 字段.
func (c *consoleEncoder) AddUint8(key string, val uint8) {
	c.AddUint64(key, uint64(val))
}

// AddUintptr 添加 uintptr 字段.
func (c *consoleEncoder) AddUintptr(key string, val uintptr) {
	c.AddUint64(key, uint64(val))
}

// AddFloat64 添加 float64 字段.
func (c *consoleEncoder) AddFloat64(key string, val float64) {
	c.addSeparator()
	formatField(c.fields, key, strconv.FormatFloat(val, 'g', -1, 64))
}

// AddFloat32 添加 float32 字段.
func (c *consoleEncoder) AddFloat32(key string, val float32) {
	c.addSeparator()
	formatField(c.fields, key, strconv.FormatFloat(float64(val), 'g', -1, 32))
}

// AddBool 添加 bool 字段.
func (c *consoleEncoder) AddBool(key string, val bool) {
	c.addSeparator()
	formatField(c.fields, key, strconv.FormatBool(val))
}

// AddDuration 添加 Duration 字段.
func (c *consoleEncoder) AddDuration(key string, val time.Duration) {
	c.addSeparator()
	formatField(c.fields, key, val.String())
}

// AddTime 添加 Time 字段.
func (c *consoleEncoder) AddTime(key string, val time.Time) {
	c.addSeparator()
	formatField(c.fields, key, val.Format("2006-01-02 15:04:05"))
}

// AddBinary 添加二进制字段.
func (c *consoleEncoder) AddBinary(key string, val []byte) {
	c.AddString(key, string(val))
}

// AddByteString 添加字节字符串字段.
func (c *consoleEncoder) AddByteString(key string, val []byte) {
	c.AddString(key, string(val))
}

// AddComplex128 添加 complex128 字段.
func (c *consoleEncoder) AddComplex128(key string, val complex128) {
	c.addSeparator()
	formatField(c.fields, key, "<complex>")
}

// AddComplex64 添加 complex64 字段.
func (c *consoleEncoder) AddComplex64(key string, val complex64) {
	c.AddComplex128(key, complex128(val))
}

// AddReflected 添加反射类型字段.
func (c *consoleEncoder) AddReflected(key string, val any) error {
	if key == ComponentKey {
		c.component = fmt.Sprint(val)
		return nil
	}
	c.addSeparator()
	var value string
	switch v := val.(type) {
	case string:
		value = v
	case error:
		value = v.Error()
	default:
		// 数组、切片、结构体等复杂类型统一使用 %v 格式化
		value = fmt.Sprint(v)
	}
	formatField(c.fields, key, value)
	return nil
}

// OpenNamespace 打开命名空间.
func (c *consoleEncoder) OpenNamespace(key string) {
	c.addSeparator()
	c.fields.AppendString(key)
	c.fields.AppendByte('.')
	c.fieldsNum++ // 保持分隔符逻辑
}

// AddArray 添加数组字段（不会被调用，Any 类型走 AddReflected）.
func (c *consoleEncoder) AddArray(key string, arr zapcore.ArrayMarshaler) error {
	c.addSeparator()
	formatField(c.fields, key, "<array>")
	return nil
}

// AddObject 添加对象字段（不会被调用，Any 类型走 AddReflected）.
func (c *consoleEncoder) AddObject(key string, obj zapcore.ObjectMarshaler) error {
	c.addSeparator()
	formatField(c.fields, key, "<object>")
	return nil
}

// addSeparator 添加字段分隔符.
func (c *consoleEncoder) addSeparator() {
	if c.fieldsNum > 0 {
		c.fields.AppendByte(' ')
	}
	c.fieldsNum++
}

// formatField 格式化字段为 [key:value] 格式.
func formatField(buf *buffer.Buffer, key, value string) {
	buf.AppendByte('[')
	buf.AppendString(key)
	buf.AppendByte(':')
	buf.AppendString(value)
	buf.AppendByte(']')
}

// encodeFieldTo 将字段编码到指定 buffer.
func (c *consoleEncoder) encodeFieldTo(buf *buffer.Buffer, field zapcore.Field) {
	var value string

	switch field.Type {
	case zapcore.StringType:
		value = field.String

	case zapcore.Int64Type, zapcore.Int32Type, zapcore.Int16Type, zapcore.Int8Type:
		value = strconv.FormatInt(field.Integer, 10)

	case zapcore.Uint64Type, zapcore.Uint32Type, zapcore.Uint16Type, zapcore.Uint8Type, zapcore.UintptrType:
		value = strconv.FormatUint(uint64(field.Integer), 10)

	case zapcore.Float64Type:
		value = strconv.FormatFloat(math.Float64frombits(uint64(field.Integer)), 'g', -1, 64)

	case zapcore.Float32Type:
		value = strconv.FormatFloat(float64(math.Float32frombits(uint32(field.Integer))), 'g', -1, 32)

	case zapcore.BoolType:
		value = strconv.FormatBool(field.Integer == 1)

	case zapcore.DurationType:
		value = time.Duration(field.Integer).String()

	case zapcore.TimeType:
		t := c.decodeTime(field)
		value = t.Format("2006-01-02 15:04:05")

	case zapcore.TimeFullType:
		if t, ok := field.Interface.(time.Time); ok {
			value = t.Format("2006-01-02 15:04:05")
		}

	case zapcore.ErrorType:
		if err, ok := field.Interface.(error); ok && err != nil {
			value = err.Error()
		}

	case zapcore.StringerType:
		if s, ok := field.Interface.(interface{ String() string }); ok {
			value = s.String()
		}

	default:
		value = c.getReflectedValue(field)
	}

	formatField(buf, field.Key, value)
}

func componentFromField(field zapcore.Field) (string, bool) {
	if field.Key != ComponentKey {
		return "", false
	}
	switch field.Type {
	case zapcore.StringType:
		return field.String, true
	default:
		if field.Interface != nil {
			return fmt.Sprint(field.Interface), true
		}
		return "", true
	}
}

func fieldsWithoutComponent(fields []zapcore.Field) []zapcore.Field {
	if len(fields) == 0 {
		return nil
	}
	result := fields[:0]
	for _, field := range fields {
		if field.Key != ComponentKey {
			result = append(result, field)
		}
	}
	return result
}

// decodeTime 解码时间字段.
func (c *consoleEncoder) decodeTime(field zapcore.Field) time.Time {
	if field.Interface != nil {
		if loc, ok := field.Interface.(*time.Location); ok {
			return time.Unix(0, field.Integer).In(loc)
		}
	}
	return time.Unix(0, field.Integer)
}

// getReflectedValue 获取反射类型字段的值.
func (c *consoleEncoder) getReflectedValue(field zapcore.Field) string {
	if field.Interface != nil {
		switch v := field.Interface.(type) {
		case string:
			return v
		case error:
			return v.Error()
		default:
			// 数组、切片、结构体等复杂类型统一使用 %v 格式化
			return fmt.Sprint(v)
		}
	}
	if field.String != "" {
		return field.String
	}
	return ""
}

// primitiveEncoder 用于编码时间、级别、调用者等基本字段.
type primitiveEncoder struct {
	buf *buffer.Buffer
}

func (e *primitiveEncoder) AppendString(v string)     { e.buf.AppendString(v) }
func (e *primitiveEncoder) AppendBool(v bool)         { e.buf.AppendString(strconv.FormatBool(v)) }
func (e *primitiveEncoder) AppendByteString(v []byte) { e.buf.AppendString(string(v)) }
func (e *primitiveEncoder) AppendInt(v int)           { e.buf.AppendString(strconv.Itoa(v)) }
func (e *primitiveEncoder) AppendInt64(v int64)       { e.buf.AppendString(strconv.FormatInt(v, 10)) }
func (e *primitiveEncoder) AppendInt32(v int32)       { e.buf.AppendString(strconv.FormatInt(int64(v), 10)) }
func (e *primitiveEncoder) AppendInt16(v int16)       { e.buf.AppendString(strconv.FormatInt(int64(v), 10)) }
func (e *primitiveEncoder) AppendInt8(v int8)         { e.buf.AppendString(strconv.FormatInt(int64(v), 10)) }
func (e *primitiveEncoder) AppendUint(v uint)         { e.buf.AppendString(strconv.FormatUint(uint64(v), 10)) }
func (e *primitiveEncoder) AppendUint64(v uint64)     { e.buf.AppendString(strconv.FormatUint(v, 10)) }
func (e *primitiveEncoder) AppendUint32(v uint32) {
	e.buf.AppendString(strconv.FormatUint(uint64(v), 10))
}
func (e *primitiveEncoder) AppendUint16(v uint16) {
	e.buf.AppendString(strconv.FormatUint(uint64(v), 10))
}
func (e *primitiveEncoder) AppendUint8(v uint8) {
	e.buf.AppendString(strconv.FormatUint(uint64(v), 10))
}
func (e *primitiveEncoder) AppendUintptr(v uintptr) {
	e.buf.AppendString(strconv.FormatUint(uint64(v), 10))
}
func (e *primitiveEncoder) AppendFloat64(v float64) {
	e.buf.AppendString(strconv.FormatFloat(v, 'g', -1, 64))
}
func (e *primitiveEncoder) AppendFloat32(v float32) {
	e.buf.AppendString(strconv.FormatFloat(float64(v), 'g', -1, 32))
}
func (e *primitiveEncoder) AppendComplex128(_ complex128)  {}
func (e *primitiveEncoder) AppendComplex64(_ complex64)    {}
func (e *primitiveEncoder) AppendDuration(v time.Duration) { e.buf.AppendString(v.String()) }
func (e *primitiveEncoder) AppendTime(v time.Time) {
	e.buf.AppendString(v.Format("2006-01-02 15:04:05"))
}
func (e *primitiveEncoder) AppendArray(_ zapcore.ArrayMarshaler) error   { return nil }
func (e *primitiveEncoder) AppendObject(_ zapcore.ObjectMarshaler) error { return nil }
func (e *primitiveEncoder) AppendReflected(v any) error {
	if s, ok := v.(interface{ String() string }); ok {
		e.buf.AppendString(s.String())
	}
	return nil
}
