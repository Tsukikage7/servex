package llm

// Role 消息角色.
type Role string

const (
	// RoleSystem 系统消息角色.
	RoleSystem Role = "system"
	// RoleUser 用户消息角色.
	RoleUser Role = "user"
	// RoleAssistant 助手消息角色.
	RoleAssistant Role = "assistant"
	// RoleTool 工具消息角色.
	RoleTool Role = "tool"
)

// ContentType 内容类型.
type ContentType string

const (
	// ContentTypeText 纯文本内容.
	ContentTypeText ContentType = "text"
	// ContentTypeImage 图片内容URL 或 base64 data URI.
	ContentTypeImage ContentType = "image"
)

// Message 对话消息.
type Message struct {
	// Role 消息角色.
	Role Role
	// Content 纯文本内容简单场景.
	Content string
	// Parts 多模态内容与 Content 互斥，Parts 不为空时优先使用.
	Parts []ContentPart
	// ToolCalls 助手请求的工具调用列表Role=assistant 时.
	ToolCalls []ToolCall
	// ToolCallID Role=tool 时，对应的工具调用 ID.
	ToolCallID string
	// Name 可选名称标识.
	Name string
}

// ContentPart 多模态内容片段.
type ContentPart struct {
	// Type 内容类型.
	Type ContentType
	// Text 文本内容Type=text 时.
	Text string
	// MediaURL 媒体 URL 或 base64 data URIType=image 时.
	MediaURL string
	// MIMEType 媒体 MIME 类型.
	MIMEType string
}

// SystemMessage 创建系统消息.
func SystemMessage(content string) Message {
	return Message{Role: RoleSystem, Content: content}
}

// UserMessage 创建用户文本消息.
func UserMessage(content string) Message {
	return Message{Role: RoleUser, Content: content}
}

// AssistantMessage 创建助手消息.
func AssistantMessage(content string) Message {
	return Message{Role: RoleAssistant, Content: content}
}

// ToolResultMessage 创建工具调用结果消息.
func ToolResultMessage(callID, content string) Message {
	return Message{Role: RoleTool, Content: content, ToolCallID: callID}
}

// UserImageMessage 创建包含文本和图片的用户消息.
func UserImageMessage(text, imageURL string) Message {
	return Message{
		Role: RoleUser,
		Parts: []ContentPart{
			{Type: ContentTypeText, Text: text},
			{Type: ContentTypeImage, MediaURL: imageURL},
		},
	}
}
