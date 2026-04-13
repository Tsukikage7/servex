// Package telegram 提供基于 Telegram Bot API 的消息通知能力.
package telegram

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/Tsukikage7/servex/v2/notify"
)

// ChannelTelegram Telegram 通知渠道.
const ChannelTelegram notify.Channel = "telegram"

// botClient 内部接口，只依赖 Send 方法，便于测试.
type botClient interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

// Sender Telegram 消息发送器，实现 notify.Sender 接口.
type Sender struct {
	client botClient
}

// NewSender 创建 Sender，内部自建 tgbotapi.BotAPI 连接.
func NewSender(token string, opts ...Option) (*Sender, error) {
	if token == "" {
		return nil, errors.New("notify/telegram: token 不能为空")
	}
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("notify/telegram: 创建 BotAPI 失败: %w", err)
	}
	s := &Sender{client: bot}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// NewSenderWithClient 使用外部已有的 tgbotapi.BotAPI 复用连接.
func NewSenderWithClient(client *tgbotapi.BotAPI) *Sender {
	return &Sender{client: client}
}

// Send 发送消息。
// msg.To 为 chat ID 列表（字符串），msg.Body 为消息正文。
// 逐一向每个 To 发送，任意一个失败则返回错误（继续发其余的）。
// Result.MessageID 为最后一条成功消息的 ID。
func (s *Sender) Send(_ context.Context, msg *notify.Message) (*notify.Result, error) {
	var lastID string
	var errs []error
	for _, to := range msg.To {
		chatID, err := strconv.ParseInt(to, 10, 64)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid chat ID %q: %w", to, err))
			continue
		}
		m := tgbotapi.NewMessage(chatID, msg.Body)
		sent, err := s.client.Send(m)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		lastID = strconv.Itoa(sent.MessageID)
	}
	if len(errs) > 0 {
		return &notify.Result{MessageID: lastID, Channel: ChannelTelegram, Error: errors.Join(errs...)}, errors.Join(errs...)
	}
	return &notify.Result{MessageID: lastID, Channel: ChannelTelegram}, nil
}

// Channel 返回渠道标识.
func (s *Sender) Channel() notify.Channel { return ChannelTelegram }

// Close 关闭连接（tgbotapi 无需显式关闭，返回 nil）.
func (s *Sender) Close() error { return nil }
