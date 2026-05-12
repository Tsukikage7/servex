package discord

import (
	"github.com/Tsukikage7/servex/v2/errors"
)

// ErrEmptyToken token 不能为空.
// ErrSessionCreate 创建 Session 失败.
var (
	ErrEmptyToken    = errors.NewWithKind(70061, "notify.discord.empty_token", "token 不能为空", errors.KindInvalidArgument)
	ErrSessionCreate = errors.NewWithKind(70062, "notify.discord.session_create", "创建 Session 失败", errors.KindInternal)
)
