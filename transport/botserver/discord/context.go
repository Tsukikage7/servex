package discord

import (
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/Tsukikage7/servex/v2/transport/botserver"
)

// discordContext 实现 botserver.Context 接口，封装单条 Discord MessageCreate 事件。
type discordContext struct {
	message *discordgo.MessageCreate
	session *discordgo.Session
	store   botserver.StateStore
	prefix  string
}

// newDiscordContext 构造 discordContext。message.Author 必须非 nil。
func newDiscordContext(
	message *discordgo.MessageCreate,
	session *discordgo.Session,
	store botserver.StateStore,
	prefix string,
) *discordContext {
	return &discordContext{
		message: message,
		session: session,
		store:   store,
		prefix:  prefix,
	}
}

// ChatID 返回频道 ID。
func (c *discordContext) ChatID() string {
	return c.message.ChannelID
}

// UserID 返回发送者 ID。
func (c *discordContext) UserID() string {
	return c.message.Author.ID
}

// Text 返回消息原始文本。
func (c *discordContext) Text() string {
	return c.message.Content
}

// Command 返回命令名不含前缀。
// 例如前缀为 "/"，消息 "/start foo" 返回 "start"；非命令返回 ""。
func (c *discordContext) Command() string {
	content := c.message.Content
	if !strings.HasPrefix(content, c.prefix) {
		return ""
	}
	// 去掉前缀后取第一个词
	trimmed := strings.TrimPrefix(content, c.prefix)
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// Args 返回命令后的参数列表；非命令返回 nil。
func (c *discordContext) Args() []string {
	content := c.message.Content
	if !strings.HasPrefix(content, c.prefix) {
		return nil
	}
	trimmed := strings.TrimPrefix(content, c.prefix)
	fields := strings.Fields(trimmed)
	if len(fields) <= 1 {
		// 仅有命令名或空，无参数
		return nil
	}
	return fields[1:]
}

// State 读取当前频道ChatID的对话状态。
func (c *discordContext) State() string {
	val, err := c.store.Get(c.message.ChannelID)
	if err != nil {
		log.Printf("component=Discord 读取状态失败 channel=%s error=%v", c.message.ChannelID, err)
		return ""
	}
	return val
}

// SetState 设置当前频道ChatID的对话状态。
func (c *discordContext) SetState(state string) {
	if err := c.store.Set(c.message.ChannelID, state); err != nil {
		log.Printf("component=Discord 设置状态失败 channel=%s error=%v", c.message.ChannelID, err)
	}
}

// Reply 向当前频道发送文本消息。
func (c *discordContext) Reply(text string, _ ...botserver.ReplyOption) error {
	_, err := c.session.ChannelMessageSend(c.message.ChannelID, text)
	return err
}

// Native 返回底层 *discordgo.MessageCreate 原始对象。
func (c *discordContext) Native() any {
	return c.message
}
