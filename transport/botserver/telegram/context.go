package telegram

import (
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/Tsukikage7/servex/v2/transport/botserver"
)

// tgContext 实现 botserver.Context 接口，封装单条 Telegram Update。
type tgContext struct {
	update *tgbotapi.Update
	client *tgbotapi.BotAPI
	store  botserver.StateStore
	chatID string
}

// newTgContext 构造 tgContext。update.Message 必须非 nil。
func newTgContext(update *tgbotapi.Update, client *tgbotapi.BotAPI, store botserver.StateStore) *tgContext {
	chatID := strconv.FormatInt(update.Message.Chat.ID, 10)
	return &tgContext{
		update: update,
		client: client,
		store:  store,
		chatID: chatID,
	}
}

// ChatID 返回字符串形式的 Chat ID。
func (c *tgContext) ChatID() string {
	return c.chatID
}

// UserID 返回字符串形式的 User ID。
func (c *tgContext) UserID() string {
	if c.update.Message.From == nil {
		return ""
	}
	return strconv.FormatInt(c.update.Message.From.ID, 10)
}

// Text 返回消息文本。
func (c *tgContext) Text() string {
	return c.update.Message.Text
}

// Command 返回命令名（不含 "/"），非命令返回 ""。
func (c *tgContext) Command() string {
	return c.update.Message.Command()
}

// Args 返回命令参数列表，无参数返回 nil。
func (c *tgContext) Args() []string {
	raw := c.update.Message.CommandArguments()
	if raw == "" {
		return nil
	}
	return strings.Fields(raw)
}

// State 读取当前 chat 的对话状态。StateStore 错误仅记录日志。
func (c *tgContext) State() string {
	val, err := c.store.Get(c.chatID)
	if err != nil {
		log.Printf("component=Telegram 读取状态失败 chat=%s error=%v", c.chatID, err)
		return ""
	}
	return val
}

// SetState 设置当前 chat 的对话状态。StateStore 错误仅记录日志。
func (c *tgContext) SetState(state string) {
	if err := c.store.Set(c.chatID, state); err != nil {
		log.Printf("component=Telegram 设置状态失败 chat=%s error=%v", c.chatID, err)
	}
}

// Reply 向当前 chat 发送文本回复。
func (c *tgContext) Reply(text string, _ ...botserver.ReplyOption) error {
	chatID := c.update.Message.Chat.ID
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := c.client.Send(msg)
	return err
}

// Native 返回底层 *tgbotapi.Update 原始对象。
func (c *tgContext) Native() any {
	return c.update
}
