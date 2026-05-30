// Package prompt 提供基于 text/template 的 AI 消息模板引擎.
package prompt

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/Tsukikage7/servex/v2/llm"
)

// Template AI 消息模板.
// 使用 Go text/template 语法，渲染后直接返回 llm.Message.
type Template struct {
	role llm.Role
	text string // 原始模板文本，持久化与版本管理需要.
	tmpl *template.Template
}

// New 创建消息模板.
// role 指定消息角色，text 为 Go text/template 格式的模板文本.
func New(role llm.Role, text string) (*Template, error) {
	tmpl, err := template.New("").Parse(text)
	if err != nil {
		return nil, fmt.Errorf("prompt: 解析模板失败: %w", err)
	}
	return &Template{role: role, text: text, tmpl: tmpl}, nil
}

// Role 返回模板的消息角色.
func (t *Template) Role() llm.Role { return t.role }

// Text 返回模板的原始文本未渲染.
func (t *Template) Text() string { return t.text }

// MustNew 创建消息模板，失败时 panic.
func MustNew(role llm.Role, text string) *Template {
	t, err := New(role, text)
	if err != nil {
		panic(err)
	}
	return t
}

// Render 使用 data 渲染模板，返回 llm.Message.
// data 可以是 struct、map 或任意 text/template 支持的数据类型.
func (t *Template) Render(data any) (llm.Message, error) {
	var buf bytes.Buffer
	if err := t.tmpl.Execute(&buf, data); err != nil {
		return llm.Message{}, fmt.Errorf("prompt: 渲染模板失败: %w", err)
	}
	return llm.Message{Role: t.role, Content: buf.String()}, nil
}

// MustRender 使用 data 渲染模板，失败时 panic.
func (t *Template) MustRender(data any) llm.Message {
	msg, err := t.Render(data)
	if err != nil {
		panic(err)
	}
	return msg
}
