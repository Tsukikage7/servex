package telegram

import (
	"github.com/Tsukikage7/servex/v2/errors"
)

// ErrEmptyToken token 不能为空.
// ErrBotAPICreate 创建 BotAPI 失败.
// ErrInvalidChatID 无效的 chat ID.
var (
	ErrEmptyToken    = errors.NewWithKind(70071, "notify.telegram.empty_token", "token 不能为空", errors.KindInvalidArgument)
	ErrBotAPICreate  = errors.NewWithKind(70072, "notify.telegram.botapi_create", "创建 BotAPI 失败", errors.KindInternal)
	ErrInvalidChatID = errors.NewWithKind(70073, "notify.telegram.invalid_chat_id", "无效的 chat ID", errors.KindInvalidArgument)
)
