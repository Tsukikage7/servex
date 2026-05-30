package response

import "github.com/Tsukikage7/servex/v2/errors"

// ExtractCode 从错误中提取错误码.
//
// 如果是 servex/errors.Error，返回对应的错误码；
// 否则返回 CodeInternal.
func ExtractCode(err error) Code {
	if err == nil {
		return CodeSuccess
	}

	// 检查是否直接是 Code 类型
	if code, ok := errors.AsType[Code](err); ok {
		return normalizeCode(code)
	}

	// 桥接 servex/errors.Error → response.Code
	if srvErr, ok := errors.FromError(err); ok {
		code := Code{
			Num:     srvErr.Code,
			Message: srvErr.Message,
			Key:     srvErr.Key,
			Kind:    srvErr.Kind,
		}
		return normalizeCode(code)
	}

	return CodeInternal
}

func normalizeCode(code Code) Code {
	if builtin, ok := lookupBuiltinCode(code.Num, code.Key); ok {
		if code.Key == "" {
			code.Key = builtin.Key
		}
		if code.Message == "" {
			code.Message = builtin.Message
		}
		if code.Kind == errors.KindInternal && code.Num != CodeInternal.Num {
			code.Kind = builtin.Kind
		}
		if code.http == 0 {
			code.http = builtin.http
		}
		return code
	}

	if code.Message == "" {
		code.Message = CodeInternal.Message
	}
	return code
}

func lookupBuiltinCode(num int, key string) (Code, bool) {
	for _, code := range builtinCodes {
		if num != 0 && code.Num == num {
			return code, true
		}
		if key != "" && code.Key == key {
			return code, true
		}
	}
	return Code{}, false
}

// ExtractMetadata 从错误中提取元数据仅 servex/errors.Error 携带 metadata.
//
// 返回 nil 表示错误无元数据或不是 servex/errors.Error.
func ExtractMetadata(err error) map[string]string {
	if err == nil {
		return nil
	}
	if srvErr, ok := errors.FromError(err); ok {
		return srvErr.Metadata
	}
	return nil
}

// ExtractMessage 从错误中提取错误消息.
//
// 对于内部错误5xxxx、6xxxx，返回通用消息，避免暴露敏感信息.
func ExtractMessage(err error) string {
	if err == nil {
		return CodeSuccess.Message
	}

	// Code 是业务明确声明的错误码，可信。
	if code, ok := errors.AsType[Code](err); ok {
		return code.Message
	}

	// 再检查 servex/errors.Error可能携带 cause 等敏感信息
	if srvErr, ok := errors.FromError(err); ok {
		if isInternalCode(srvErr.Code) {
			return CodeInternal.Message
		}
		return srvErr.Message
	}

	return CodeInternal.Message
}

// isInternalCode 判断是否为内部/外部服务错误应掩码 Message.
// 规范：5xxxx=服务器内部，6xxxx=外部服务；业务码 >= 70000 不掩码.
func isInternalCode(code int) bool {
	return code >= 50000 && code < 70000
}

// ExtractMessageUnsafe 从错误中提取完整错误消息包含敏感信息.
//
// 仅用于日志记录，不应返回给客户端.
func ExtractMessageUnsafe(err error) string {
	if err == nil {
		return CodeSuccess.Message
	}

	return err.Error()
}
