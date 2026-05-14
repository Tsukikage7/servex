// Package discord 提供基于 Discord Bot API 的消息通知能力.
package discord

import (
	"context"
	"errors"

	"github.com/bwmarrin/discordgo"

	"github.com/Tsukikage7/servex/v2/notify"
)

// ChannelDiscord Discord 通知渠道.
const ChannelDiscord notify.Channel = "discord"

// discordClient 内部接口，隔离 discordgo 依赖便于测试.
type discordClient interface {
	ChannelMessageSend(channelID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
	Close() error
}

// Sender Discord 消息发送器，实现 notify.Sender 接口.
type Sender struct {
	client discordClient
}

// NewSender 创建 Sender，内部创建 discordgo.Session。
// token 为 Bot Token（不含 "Bot " 前缀，内部自动添加）。
func NewSender(token string, opts ...Option) (*Sender, error) {
	if token == "" {
		return nil, ErrEmptyToken
	}
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, ErrSessionCreate.WithCause(err)
	}
	// 注意：只使用 HTTP API 发消息，不需要建立 Gateway 连接（无需 Open）
	s := &Sender{client: session}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// NewSenderWithClient 使用外部已有的 discordgo.Session 复用连接。
// 注意：*discordgo.Session 实现了 discordClient 接口。
func NewSenderWithClient(session *discordgo.Session) *Sender {
	return &Sender{client: session}
}

// Send 发送消息。
// msg.To 为 channel ID 列表，msg.Body 为消息正文。
// 逐一向每个 To 发送，任意失败继续发其余的，最终汇总错误。
// Result.MessageID 为最后一条成功消息的 ID。
func (s *Sender) Send(_ context.Context, msg *notify.Message) (*notify.Result, error) {
	var lastID string
	var errs []error
	for _, channelID := range msg.To {
		sent, err := s.client.ChannelMessageSend(channelID, msg.Body)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		lastID = sent.ID
	}
	if len(errs) > 0 {
		return &notify.Result{MessageID: lastID, Channel: ChannelDiscord, Error: errors.Join(errs...)}, errors.Join(errs...)
	}
	return &notify.Result{MessageID: lastID, Channel: ChannelDiscord}, nil
}

// Channel 返回渠道标识.
func (s *Sender) Channel() notify.Channel { return ChannelDiscord }

// Close 关闭连接.
func (s *Sender) Close() error { return s.client.Close() }
