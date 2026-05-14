# i18n

`github.com/Tsukikage7/servex/v2/i18n` -- 国际化。

## 概述

i18n 包提供国际化本地化支持，基于 `golang.org/x/text/language` 实现语言匹配，使用 JSON 文件存储翻译消息，支持 `text/template` 模板语法进行参数替换。

## 功能特性

- 基于 BCP 47 语言标签的智能语言匹配
- 支持从 JSON 文件加载翻译消息
- 支持直接注册消息映射
- 使用 `text/template` 模板语法进行参数替换
- 消息回退机制：匹配语言 -> 默认语言 -> messageID

## API

### 类型

| 类型 | 说明 |
|------|------|
| `Bundle` | 消息包，管理多语言消息文件与 Matcher |
| `Localizer` | 本地化器，用于翻译消息 |

### Bundle 方法

| 方法 | 说明 |
|------|------|
| `NewBundle(defaultLang, opts...) *Bundle` | 创建消息包 |
| `LoadMessageFile(tag, path) error` | 从 JSON 文件加载翻译消息 |
| `LoadMessages(tag, messages)` | 直接注册消息映射 |
| `NewLocalizer(langs ...string) *Localizer` | 创建本地化器，langs 按优先级排列 |

### Localizer 方法

| 方法 | 说明 |
|------|------|
| `Translate(messageID, data...) string` | 翻译消息，未找到时返回 messageID |
| `MustTranslate(messageID, defaultMsg, data...) string` | 翻译消息，未找到时返回 defaultMsg |

### 配置选项

| 选项 | 说明 |
|------|------|
| `WithLogger(logger)` | 设置日志记录器 |
