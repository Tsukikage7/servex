// Package telegram 提供基于 Telegram Bot API 的消息通知能力.
package telegram

// Option Sender 配置选项函数，预留扩展点.
type Option func(*Sender)
