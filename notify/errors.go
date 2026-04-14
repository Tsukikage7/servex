package notify

import (
	"net/http"

	"github.com/Tsukikage7/servex/v2/errors"
	"google.golang.org/grpc/codes"
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
	ErrNilMessage       = errors.New(70001, "notify.nil_message", "消息为空").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)
	ErrEmptyChannel     = errors.New(70002, "notify.empty_channel", "渠道为空").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)
	ErrInvalidChannel   = errors.New(70003, "notify.invalid_channel", "无效渠道").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)
	ErrEmptyRecipients  = errors.New(70004, "notify.empty_recipients", "收件人为空").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)
	ErrNoSender         = errors.New(70005, "notify.no_sender", "未找到对应渠道的 Sender").WithHTTP(http.StatusNotFound).WithGRPC(codes.NotFound)
	ErrClosed           = errors.New(70006, "notify.closed", "分发器已关闭").WithHTTP(http.StatusServiceUnavailable).WithGRPC(codes.Unavailable)
	ErrTemplateNotFound = errors.New(70007, "notify.template_not_found", "模板未找到").WithHTTP(http.StatusNotFound).WithGRPC(codes.NotFound)
	ErrTemplateRender   = errors.New(70008, "notify.template_render", "模板渲染失败").WithHTTP(http.StatusInternalServerError).WithGRPC(codes.Internal)
	ErrNotImplemented   = errors.New(70009, "notify.not_implemented", "未实现").WithHTTP(http.StatusNotImplemented).WithGRPC(codes.Unimplemented)

	ErrJobQueueNotConfigured = errors.New(70010, "notify.jobqueue_not_configured", "jobqueue 未配置").WithHTTP(http.StatusInternalServerError).WithGRPC(codes.FailedPrecondition)
	ErrSerializeFailed       = errors.New(70011, "notify.serialize_failed", "序列化消息失败").WithHTTP(http.StatusInternalServerError).WithGRPC(codes.Internal)
)
