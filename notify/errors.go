package notify

import (
	"github.com/Tsukikage7/servex/v2/errors"
)

// ErrNilMessage 消息为空.
// ErrEmptyChannel 渠道为空.
// ErrInvalidChannel 无效渠道.
// ErrEmptyRecipients 收件人为空.
// ErrNoSender 未找到对应渠道的 Sender.
// ErrClosed 分发器已关闭.
// ErrTemplateNotFound 模板未找到.
// ErrTemplateRender 模板渲染失败.
// ErrNotImplemented 未实现.
// ErrJobQueueNotConfigured jobqueue 未配置.
// ErrSerializeFailed 序列化消息失败.
var (
	ErrNilMessage       = errors.NewWithKind(70001, "notify.nil_message", "消息为空", errors.KindInvalidArgument)
	ErrEmptyChannel     = errors.NewWithKind(70002, "notify.empty_channel", "渠道为空", errors.KindInvalidArgument)
	ErrInvalidChannel   = errors.NewWithKind(70003, "notify.invalid_channel", "无效渠道", errors.KindInvalidArgument)
	ErrEmptyRecipients  = errors.NewWithKind(70004, "notify.empty_recipients", "收件人为空", errors.KindInvalidArgument)
	ErrNoSender         = errors.NewWithKind(70005, "notify.no_sender", "未找到对应渠道的 Sender", errors.KindNotFound)
	ErrClosed           = errors.NewWithKind(70006, "notify.closed", "分发器已关闭", errors.KindUnavailable)
	ErrTemplateNotFound = errors.NewWithKind(70007, "notify.template_not_found", "模板未找到", errors.KindNotFound)
	ErrTemplateRender   = errors.NewWithKind(70008, "notify.template_render", "模板渲染失败", errors.KindInternal)
	ErrNotImplemented   = errors.NewWithKind(70009, "notify.not_implemented", "未实现", errors.KindNotImplemented)

	ErrJobQueueNotConfigured = errors.NewWithKind(70010, "notify.jobqueue_not_configured", "jobqueue 未配置", errors.KindFailedPrecondition)
	ErrSerializeFailed       = errors.NewWithKind(70011, "notify.serialize_failed", "序列化消息失败", errors.KindInternal)
)
