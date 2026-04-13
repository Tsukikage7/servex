// Package nwebhook 提供通知渠道的 Webhook 发送能力.
package nwebhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/Tsukikage7/servex/v2/notify"
)

// Sender 通知 Webhook 发送器.
type Sender struct {
	opts   senderOptions
	closed atomic.Bool
}

// NewSender 创建通知 Webhook 发送器实例.
func NewSender(opts ...Option) (*Sender, error) {
	o := senderOptions{timeout: 10 * time.Second}
	for _, opt := range opts {
		opt(&o)
	}
	if o.httpClient == nil {
		o.httpClient = &http.Client{Timeout: o.timeout}
	}
	return &Sender{opts: o}, nil
}

// Channel 返回 Webhook 渠道标识.
func (s *Sender) Channel() notify.Channel { return notify.ChannelWebhook }

// Send 发送 Webhook 通知消息.
func (s *Sender) Send(ctx context.Context, msg *notify.Message) (*notify.Result, error) {
	if msg == nil {
		return nil, notify.ErrNilMessage
	}
	if s.closed.Load() {
		return nil, notify.ErrClosed
	}

	url := msg.To[0]
	format := msg.Metadata["format"]
	var payload []byte
	if format == "custom" || format == "" {
		payload = []byte(msg.Body)
	} else {
		payload = getFormatter(format)(msg.Subject, msg.Body)
	}
	secret := msg.Metadata["secret"]
	msgID := uuid.New().String()

	var lastErr error
	for attempt := 0; attempt <= s.opts.maxRetry; attempt++ {
		if err := s.doSend(ctx, url, payload, secret); err == nil {
			return &notify.Result{MessageID: msgID, Channel: notify.ChannelWebhook}, nil
		} else {
			lastErr = err
		}
	}
	return nil, lastErr
}

func (s *Sender) doSend(ctx context.Context, url string, payload []byte, secret string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		req.Header.Set("X-Signature", hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := s.opts.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("notification/webhook: 投递失败，状态码 %d", resp.StatusCode)
	}
	return nil
}

// Close 关闭通知 Webhook 发送器.
func (s *Sender) Close() error { s.closed.Store(true); return nil }
