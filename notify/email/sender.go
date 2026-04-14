// Package email 提供基于 SMTP 的邮件发送能力.
package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/Tsukikage7/servex/v2/notify"
)

// Sender 邮件发送器.
type Sender struct {
	opts   senderOptions
	closed atomic.Bool
}

// NewSender 创建邮件发送器实例.
func NewSender(opts ...Option) (*Sender, error) {
	var o senderOptions
	for _, opt := range opts {
		opt(&o)
	}
	if o.host == "" {
		return nil, ErrEmptySMTPHost
	}
	if o.fromAddr == "" {
		return nil, ErrEmptyFromAddr
	}
	return &Sender{opts: o}, nil
}

// Channel 返回邮件渠道标识.
func (s *Sender) Channel() notify.Channel { return notify.ChannelEmail }

// Send 发送邮件消息.
func (s *Sender) Send(ctx context.Context, msg *notify.Message) (*notify.Result, error) {
	if msg == nil {
		return nil, notify.ErrNilMessage
	}
	if s.closed.Load() {
		return nil, notify.ErrClosed
	}

	msgID := uuid.New().String()
	recipients := append([]string{}, msg.To...)

	var ccList, bccList []string
	if cc := msg.Metadata["cc"]; cc != "" {
		ccList = strings.Split(cc, ",")
		recipients = append(recipients, ccList...)
	}
	if bcc := msg.Metadata["bcc"]; bcc != "" {
		bccList = strings.Split(bcc, ",")
		recipients = append(recipients, bccList...)
	}

	var buf strings.Builder
	fromHeader := s.opts.fromAddr
	if s.opts.fromName != "" {
		fromHeader = mime.QEncoding.Encode("utf-8", s.opts.fromName) + " <" + s.opts.fromAddr + ">"
	}
	buf.WriteString("From: " + fromHeader + "\r\n")
	buf.WriteString("To: " + strings.Join(msg.To, ",") + "\r\n")
	if len(ccList) > 0 {
		buf.WriteString("Cc: " + strings.Join(ccList, ",") + "\r\n")
	}
	buf.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", msg.Subject) + "\r\n")
	buf.WriteString("Message-ID: <" + msgID + ">\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")
	if replyTo := msg.Metadata["reply_to"]; replyTo != "" {
		buf.WriteString("Reply-To: " + replyTo + "\r\n")
	}
	buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	buf.WriteString(msg.Body)

	addr := fmt.Sprintf("%s:%d", s.opts.host, s.opts.port)
	if err := s.sendMail(addr, recipients, []byte(buf.String())); err != nil {
		return nil, err
	}
	return &notify.Result{MessageID: msgID, Channel: notify.ChannelEmail}, nil
}

func (s *Sender) sendMail(addr string, recipients []string, body []byte) error {
	host, _, _ := net.SplitHostPort(addr)
	var client *smtp.Client
	var err error

	if s.opts.useTLS {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
		if err != nil {
			return fmt.Errorf("notification/email: TLS 连接失败: %w", err)
		}
		client, err = smtp.NewClient(conn, host)
		if err != nil {
			conn.Close()
			return fmt.Errorf("notification/email: 创建客户端失败: %w", err)
		}
	} else {
		client, err = smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("notification/email: 连接失败: %w", err)
		}
	}
	defer client.Close()

	if s.opts.username != "" {
		auth := smtp.PlainAuth("", s.opts.username, s.opts.password, host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("notification/email: 认证失败: %w", err)
		}
	}

	if err := client.Mail(s.opts.fromAddr); err != nil {
		return err
	}
	for _, rcpt := range recipients {
		if err := client.Rcpt(strings.TrimSpace(rcpt)); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	w.Write(body)
	w.Close()
	return client.Quit()
}

// Close 关闭邮件发送器.
func (s *Sender) Close() error { s.closed.Store(true); return nil }
