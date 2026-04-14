package sms

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/Tsukikage7/servex/v2/notify"
)

// Sender 短信发送器.
type Sender struct {
	provider Provider
	opts     senderOptions
	closed   atomic.Bool
}

// NewSender 创建短信发送器实例.
func NewSender(provider Provider, opts ...Option) (*Sender, error) {
	if provider == nil {
		return nil, ErrNilProvider
	}
	var o senderOptions
	for _, opt := range opts {
		opt(&o)
	}
	return &Sender{provider: provider, opts: o}, nil
}

// Channel 返回短信渠道标识.
func (s *Sender) Channel() notify.Channel { return notify.ChannelSMS }

// Send 向目标手机号发送短信.
func (s *Sender) Send(ctx context.Context, msg *notify.Message) (*notify.Result, error) {
	if msg == nil {
		return nil, notify.ErrNilMessage
	}
	if s.closed.Load() {
		return nil, notify.ErrClosed
	}

	params := make(map[string]string, len(msg.TemplateData))
	for k, v := range msg.TemplateData {
		params[k] = fmt.Sprintf("%v", v)
	}

	var lastID string
	var errs []string
	for _, phone := range msg.To {
		id, err := s.provider.Send(ctx, &SendRequest{
			Phone: phone, Content: msg.Body, SignName: s.opts.signName,
			TemplateCode: msg.TemplateID, Params: params,
		})
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", phone, err))
			continue
		}
		lastID = id
	}
	if len(errs) > 0 {
		return nil, ErrPartialSendFailed.WithMessage("部分发送失败: " + strings.Join(errs, "; "))
	}
	return &notify.Result{MessageID: lastID, Channel: notify.ChannelSMS}, nil
}

// Close 关闭短信发送器.
func (s *Sender) Close() error { s.closed.Store(true); return nil }
