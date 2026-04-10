package notify

import "errors"

// ErrNilMessage 消息为空.
// ErrEmptyChannel 渠道为空.
// ErrInvalidChannel 无效渠道.
// ErrEmptyRecipients 收件人为空.
// ErrNoSender 未找到对应渠道的 Sender.
// ErrClosed 分发器已关闭.
// ErrTemplateNotFound 模板未找到.
// ErrTemplateRender 模板渲染失败.
var (
	ErrNilMessage       = errors.New("notification: 消息为空")
	ErrEmptyChannel     = errors.New("notification: 渠道为空")
	ErrInvalidChannel   = errors.New("notification: 无效渠道")
	ErrEmptyRecipients  = errors.New("notification: 收件人为空")
	ErrNoSender         = errors.New("notification: 未找到对应渠道的 Sender")
	ErrClosed           = errors.New("notification: 已关闭")
	ErrTemplateNotFound = errors.New("notification: 模板未找到")
	ErrTemplateRender   = errors.New("notification: 模板渲染失败")
	ErrNotImplemented   = errors.New("notification: 未实现")
)
