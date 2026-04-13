package response_test

import (
	"errors"
	"testing"

	"golang.org/x/text/language"

	"github.com/Tsukikage7/servex/v2/i18n"
	"github.com/Tsukikage7/servex/v2/transport/response"
)

func TestLocalizedMessage_Nil(t *testing.T) {
	got := response.LocalizedMessage(nil)
	if got != "成功" {
		t.Errorf("got %q, want %q", got, "成功")
	}
}

func TestLocalizedMessage_DefaultChinese(t *testing.T) {
	err := response.NewError(response.CodeNotFound)
	got := response.LocalizedMessage(err)
	if got != "资源不存在" {
		t.Errorf("got %q, want %q", got, "资源不存在")
	}
}

func TestLocalizedMessage_English(t *testing.T) {
	err := response.NewError(response.CodeNotFound)
	got := response.LocalizedMessage(err, "en")
	if got != "Resource not found" {
		t.Errorf("got %q, want %q", got, "Resource not found")
	}
}

func TestLocalizedMessage_CustomMessage(t *testing.T) {
	err := response.NewErrorWithMessage(response.CodeInvalidParam, "用户名不能为空")
	// 业务层自定义消息优先于 i18n 翻译
	got := response.LocalizedMessage(err, "en")
	if got != "用户名不能为空" {
		t.Errorf("got %q, want %q", got, "用户名不能为空")
	}
}

func TestLocalizedMessage_InternalError_HidesDetail(t *testing.T) {
	bizErr := response.NewErrorFull(response.CodeInternal, "敏感数据库信息", errors.New("sql: no rows"))
	got := response.LocalizedMessage(bizErr)
	// 5xxxx 错误隐藏细节，返回 Code.Key 的翻译
	if got != "服务器内部错误" {
		t.Errorf("got %q, want %q", got, "服务器内部错误")
	}
	gotEN := response.LocalizedMessage(bizErr, "en")
	if gotEN != "Internal server error" {
		t.Errorf("got %q, want %q", gotEN, "Internal server error")
	}
}

func TestLocalizedMessage_AcceptLanguageHeader(t *testing.T) {
	err := response.NewError(response.CodeUnauthorized)
	// 模拟 Accept-Language: en-US,en;q=0.9
	got := response.LocalizedMessage(err, "en-US,en;q=0.9")
	if got != "Unauthorized" {
		t.Errorf("got %q, want %q", got, "Unauthorized")
	}
}

func TestLocalizedMessage_FallbackToCode_WhenNoKey(t *testing.T) {
	customCode := response.Code{
		Num:        99001,
		Message:    "自定义错误",
		HTTPStatus: 400,
		// 没有设置 Key
	}
	err := response.NewError(customCode)
	got := response.LocalizedMessage(err, "en")
	// 无 Key 时直接返回 Message
	if got != "自定义错误" {
		t.Errorf("got %q, want %q", got, "自定义错误")
	}
}

func TestSetBundle_CustomBundle(t *testing.T) {
	orig := response.GetBundle()
	defer response.SetBundle(orig) // 恢复

	custom := i18n.NewBundle(language.Chinese)
	custom.LoadMessages(language.Chinese, map[string]string{
		"error.not_found": "找不到该资源（自定义）",
	})
	custom.LoadMessages(language.English, map[string]string{
		"error.not_found": "Custom: resource not found",
	})
	response.SetBundle(custom)

	err := response.NewError(response.CodeNotFound)
	if got := response.LocalizedMessage(err, "zh"); got != "找不到该资源（自定义）" {
		t.Errorf("zh: got %q", got)
	}
	if got := response.LocalizedMessage(err, "en"); got != "Custom: resource not found" {
		t.Errorf("en: got %q", got)
	}
}
