package telegram

import (
	"net/http"

	"github.com/Tsukikage7/servex/v2/errors"
	"google.golang.org/grpc/codes"
)

// ErrEmptyToken token 不能为空.
// ErrBotAPICreate 创建 BotAPI 失败.
// ErrInvalidChatID 无效的 chat ID.
var (
	ErrEmptyToken   = errors.New(70071, "notify.telegram.empty_token", "token 不能为空").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)
	ErrBotAPICreate = errors.New(70072, "notify.telegram.botapi_create", "创建 BotAPI 失败").WithHTTP(http.StatusInternalServerError).WithGRPC(codes.Internal)
	ErrInvalidChatID = errors.New(70073, "notify.telegram.invalid_chat_id", "无效的 chat ID").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)
)
