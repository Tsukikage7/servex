package response

import (
	"errors"
	"sync/atomic"

	"golang.org/x/text/language"

	servexerr "github.com/Tsukikage7/servex/v2/errors"
	"github.com/Tsukikage7/servex/v2/i18n"
)

// globalBundle 全局消息包，默认内置中文 + 英文翻译.
//
// 可通过 SetBundle 替换为用户自定义的多语言包.
var globalBundle atomic.Pointer[i18n.Bundle]

func init() {
	b := newBuiltinBundle()
	globalBundle.Store(b)
}

// newBuiltinBundle 构建内置消息包（中文为默认语言）.
func newBuiltinBundle() *i18n.Bundle {
	b := i18n.NewBundle(language.Chinese)
	b.LoadMessages(language.Chinese, builtinZH)
	b.LoadMessages(language.English, builtinEN)
	return b
}

// SetBundle 替换全局消息包.
//
// 用于应用启动时注册自定义翻译（追加或覆盖内置语言）：
//
//	bundle := i18n.NewBundle(language.Chinese)
//	bundle.LoadMessages(language.Chinese, zhMessages)
//	bundle.LoadMessages(language.English, enMessages)
//	response.SetBundle(bundle)
func SetBundle(b *i18n.Bundle) { globalBundle.Store(b) }

// GetBundle 获取当前全局消息包.
func GetBundle() *i18n.Bundle { return globalBundle.Load() }

// LocalizedMessage 获取本地化后的错误消息.
//
// langs 为语言偏好列表，通常直接传入 Accept-Language 请求头的值。
// 翻译规则（优先级从高到低）：
//  1. 内部错误（5xxxx+）始终使用 Code 级别消息，不透传业务细节
//  2. BusinessError/*errors.Error 含自定义消息时直接返回（已由业务层明确指定）
//  3. 其余情况通过全局 Bundle 翻译 Code.Key，未命中时回退到 Code.Message
func LocalizedMessage(err error, langs ...string) string {
	if err == nil {
		return localizeCode(CodeSuccess, langs...)
	}

	// BusinessError：内部错误（5xxxx+）掩码；否则自定义消息优先于 i18n
	if bizErr, ok := errors.AsType[*BusinessError](err); ok {
		if isInternalCode(bizErr.Code.Num) {
			return localizeCode(bizErr.Code, langs...)
		}
		if bizErr.Message != "" {
			return bizErr.Message
		}
		return localizeCode(bizErr.Code, langs...)
	}

	// servex/errors.Error：内部错误掩码；否则翻译 Key，回退到 Message
	if srvErr, ok := servexerr.FromError(err); ok {
		if isInternalCode(srvErr.Code) {
			return localizeCode(CodeInternal, langs...)
		}
		if srvErr.Key != "" {
			if translated := localizeCode(Code{Key: srvErr.Key, Message: srvErr.Message}, langs...); translated != srvErr.Key {
				return translated
			}
		}
		return srvErr.Message
	}

	// 其他场景走 Code 翻译
	return localizeCode(ExtractCode(err), langs...)
}

// localizeCode 翻译单个 Code 的消息键，回退到 Code.Message.
//
// langs 可直接传入 Accept-Language 头的原始值（如 "en-US,en;q=0.9"），
// 内部使用 language.ParseAcceptLanguage 解析，保留 q 值优先级顺序.
func localizeCode(code Code, langs ...string) string {
	b := globalBundle.Load()
	if b == nil || code.Key == "" {
		return code.Message
	}
	// 解析 Accept-Language 头中的语言标签（保持优先级顺序）
	parsed := parseAcceptLangs(langs)
	loc := b.NewLocalizer(parsed...)
	return loc.MustTranslate(code.Key, code.Message)
}

// parseAcceptLangs 将 Accept-Language 头字符串列表解析为语言标签字符串列表.
//
// 输入可以是标准 Accept-Language 头值（"zh-CN,zh;q=0.9,en;q=0.8"）
// 或单个语言标签（"en"），二者均可正确处理.
func parseAcceptLangs(langs []string) []string {
	result := make([]string, 0, len(langs)*2)
	for _, l := range langs {
		// 尝试解析完整 Accept-Language 头（含 q 值）
		tags, _, err := language.ParseAcceptLanguage(l)
		if err == nil && len(tags) > 0 {
			for _, t := range tags {
				result = append(result, t.String())
			}
		} else {
			// 单个语言标签，直接保留
			result = append(result, l)
		}
	}
	return result
}

// builtinZH 内置中文错误消息.
var builtinZH = map[string]string{
	"success":               "成功",
	"error.unknown":         "未知错误",
	"error.canceled":        "请求已取消",
	"error.timeout":         "请求超时",
	"error.unauthorized":    "未授权",
	"error.forbidden":       "禁止访问",
	"error.token_expired":   "令牌已过期",
	"error.token_invalid":   "令牌无效",
	"error.invalid_param":   "参数无效",
	"error.missing_param":   "缺少必需参数",
	"error.validation":      "参数验证失败",
	"error.not_found":       "资源不存在",
	"error.already_exists":  "资源已存在",
	"error.conflict":        "资源冲突",
	"error.exhausted":       "资源耗尽",
	"error.internal":        "服务器内部错误",
	"error.not_implemented": "功能未实现",
	"error.database":        "数据库错误",
	"error.unavailable":     "服务不可用",
	"error.upstream":        "上游服务错误",
}

// builtinEN 内置英文错误消息.
var builtinEN = map[string]string{
	"success":               "Success",
	"error.unknown":         "Unknown error",
	"error.canceled":        "Request canceled",
	"error.timeout":         "Request timeout",
	"error.unauthorized":    "Unauthorized",
	"error.forbidden":       "Forbidden",
	"error.token_expired":   "Token expired",
	"error.token_invalid":   "Invalid token",
	"error.invalid_param":   "Invalid parameter",
	"error.missing_param":   "Missing required parameter",
	"error.validation":      "Validation failed",
	"error.not_found":       "Resource not found",
	"error.already_exists":  "Resource already exists",
	"error.conflict":        "Resource conflict",
	"error.exhausted":       "Resource exhausted",
	"error.internal":        "Internal server error",
	"error.not_implemented": "Not implemented",
	"error.database":        "Database error",
	"error.unavailable":     "Service unavailable",
	"error.upstream":        "Upstream service error",
}
