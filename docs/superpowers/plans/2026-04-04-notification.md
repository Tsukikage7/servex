# notification/ 多渠道通知服务 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 统一接口抽象四个渠道（Email/SMS/Webhook/Push），内置模板引擎，支持异步发送（可选集成 jobqueue）。

**Architecture:** Dispatcher 调度器管理多个 Sender，按 Channel 路由消息。TemplateEngine 在发送前渲染模板。SendAsync 通过 jobqueue 实现异步投递。子包隔离模式与 pubsub/ 一致。

**Tech Stack:** Go 标准库 net/smtp、net/http、html/template、crypto/hmac；servex 内部包 logger、errors、jobqueue

---

# TDD Implementation Plan: `notification/` Package

## Conventions Observed from Codebase

From examining the existing packages (`pubsub/`, `webhook/`, `i18n/`, `jobqueue/`, `pubsub/kafka/`, `pubsub/redis/`):

- **Options pattern**: private struct + `func(*opts)` (e.g., `/Users/tsukikage/workspace/work/servex/webhook/options.go` lines 9-16, `/Users/tsukikage/workspace/work/servex/pubsub/kafka/options.go` lines 6-8)
- **Errors**: `errors.go` with `errors.New("pkg: 中文描述")` sentinel errors (e.g., `/Users/tsukikage/workspace/work/servex/pubsub/errors.go`)
- **Tests**: same-package tests using stdlib `testing` with `t.Fatal`/`t.Errorf` + `errors.Is` checks (e.g., `/Users/tsukikage/workspace/work/servex/pubsub/kafka/publisher_test.go`); interface compliance via `var _ Interface = (*Concrete)(nil)`
- **Constructors**: return concrete pointer, not interface (e.g., `webhook.NewDispatcher` returns `*dispatcher`)
- **Chinese comments and error messages** throughout
- **Commit format**: `feat(notification): 描述`
- **Logger**: `logger.Logger` interface with field helpers `logger.String()`, `logger.Err()`, `logger.Duration()` (see `/Users/tsukikage/workspace/work/servex/logger/zap.go` lines 367-455)
- **Factory**: separate `factory/` sub-package to avoid circular deps, with `Config` struct holding per-driver configs (see `/Users/tsukikage/workspace/work/servex/pubsub/factory/factory.go`)
- **Module**: `github.com/Tsukikage7/servex`

---

## Task 1: Core Types -- `notification.go` + `errors.go`

### Files to Create
- `notification/notification.go`
- `notification/errors.go`
- `notification/notification_test.go`

### Test Code -- `notification/notification_test.go`

```go
// notification/notification_test.go
package notification

import (
	"errors"
	"testing"
)

func TestChannel_String(t *testing.T) {
	tests := []struct {
		ch   Channel
		want string
	}{
		{ChannelEmail, "email"},
		{ChannelSMS, "sms"},
		{ChannelWebhook, "webhook"},
		{ChannelPush, "push"},
	}
	for _, tt := range tests {
		if got := string(tt.ch); got != tt.want {
			t.Errorf("Channel = %q, want %q", got, tt.want)
		}
	}
}

func TestChannel_Valid(t *testing.T) {
	if !ChannelEmail.Valid() {
		t.Error("email should be valid")
	}
	if Channel("fax").Valid() {
		t.Error("fax should not be valid")
	}
}

func TestMessage_Validate(t *testing.T) {
	tests := []struct {
		name string
		msg  *Message
		err  error
	}{
		{"nil message", nil, ErrNilMessage},
		{"empty channel", &Message{}, ErrEmptyChannel},
		{"invalid channel", &Message{Channel: "fax", To: []string{"x"}}, ErrInvalidChannel},
		{"empty recipients", &Message{Channel: ChannelEmail}, ErrEmptyRecipients},
		{"valid", &Message{Channel: ChannelEmail, To: []string{"a@b.com"}, Body: "hi"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMessage(tt.msg)
			if !errors.Is(err, tt.err) {
				t.Errorf("got %v, want %v", err, tt.err)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	errs := []error{
		ErrNilMessage, ErrEmptyChannel, ErrInvalidChannel,
		ErrEmptyRecipients, ErrNoSender, ErrClosed,
		ErrTemplateNotFound, ErrTemplateRender,
	}
	for _, e := range errs {
		if e == nil {
			t.Error("sentinel error should not be nil")
		}
	}
}
```

### Implementation Code -- `notification/notification.go`

```go
// Package notification 提供统一的多渠道通知发送能力。
package notification

import "context"

// Channel 渠道枚举。
type Channel string

const (
	ChannelEmail   Channel = "email"
	ChannelSMS     Channel = "sms"
	ChannelWebhook Channel = "webhook"
	ChannelPush    Channel = "push"
)

var validChannels = map[Channel]bool{
	ChannelEmail: true, ChannelSMS: true,
	ChannelWebhook: true, ChannelPush: true,
}

// Valid 返回渠道是否合法。
func (c Channel) Valid() bool { return validChannels[c] }

// Message 统一消息体。
type Message struct {
	Channel      Channel
	To           []string
	Subject      string
	Body         string
	TemplateID   string
	TemplateData map[string]any
	Metadata     map[string]string
}

// Result 发送结果。
type Result struct {
	MessageID string
	Channel   Channel
	Error     error
}

// Sender 统一发送接口。
type Sender interface {
	Send(ctx context.Context, msg *Message) (*Result, error)
	Channel() Channel
	Close() error
}

// TemplateEngine 模板渲染接口。
type TemplateEngine interface {
	Render(templateID string, data map[string]any) (string, error)
}

// ValidateMessage 校验消息基本字段。
func ValidateMessage(msg *Message) error {
	if msg == nil {
		return ErrNilMessage
	}
	if msg.Channel == "" {
		return ErrEmptyChannel
	}
	if !msg.Channel.Valid() {
		return ErrInvalidChannel
	}
	if len(msg.To) == 0 {
		return ErrEmptyRecipients
	}
	return nil
}
```

### Implementation Code -- `notification/errors.go`

```go
// notification/errors.go
package notification

import "errors"

var (
	ErrNilMessage       = errors.New("notification: 消息为空")
	ErrEmptyChannel     = errors.New("notification: 渠道为空")
	ErrInvalidChannel   = errors.New("notification: 无效渠道")
	ErrEmptyRecipients  = errors.New("notification: 收件人为空")
	ErrNoSender         = errors.New("notification: 未找到对应渠道的 Sender")
	ErrClosed           = errors.New("notification: 已关闭")
	ErrTemplateNotFound = errors.New("notification: 模板未找到")
	ErrTemplateRender   = errors.New("notification: 模板渲染失败")
)
```

### Test Command
```bash
go test ./notification/ -run "TestChannel|TestMessage|TestErrors" -v
```

### Commit Message
```
feat(notification): Channel/Message/Result/Sender 核心类型与包级错误
```

---

## Task 2: Template Engine -- `template.go`

### Files to Create
- `notification/template.go`
- `notification/template_test.go`
- `notification/testdata/templates/greeting.html` (containing `<p>Hello, {{.User}}!</p>`)

### Test Code -- `notification/template_test.go`

```go
// notification/template_test.go
package notification

import (
	"embed"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

//go:embed testdata/templates
var testFS embed.FS

func TestTemplateEngine_RenderNotFound(t *testing.T) {
	eng := NewTemplateEngine()
	_, err := eng.Render("nonexistent", nil)
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Errorf("got %v, want ErrTemplateNotFound", err)
	}
}

func TestTemplateEngine_WithTemplateDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "welcome.html"), []byte(`<h1>Hello, {{.Name}}!</h1>`), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := NewTemplateEngine(WithTemplateDir(dir))
	got, err := eng.Render("welcome.html", map[string]any{"Name": "Alice"})
	if err != nil {
		t.Fatal(err)
	}
	if want := `<h1>Hello, Alice!</h1>`; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTemplateEngine_WithTemplateFS(t *testing.T) {
	eng := NewTemplateEngine(WithTemplateFS(testFS, "testdata/templates"))
	got, err := eng.Render("greeting.html", map[string]any{"User": "Bob"})
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Error("expected non-empty rendered output")
	}
}

func TestTemplateEngine_RenderError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.html"), []byte(`{{.Name`), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := NewTemplateEngine(WithTemplateDir(dir))
	_, err := eng.Render("bad.html", nil)
	if err == nil {
		t.Error("expected error for bad template")
	}
}

func TestTemplateEngine_NilData(t *testing.T) {
	dir := t.TempDir()
	content := `<p>Static</p>`
	if err := os.WriteFile(filepath.Join(dir, "static.html"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := NewTemplateEngine(WithTemplateDir(dir))
	got, err := eng.Render("static.html", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != content {
		t.Errorf("got %q, want %q", got, content)
	}
}
```

### Implementation Code -- `notification/template.go`

```go
// notification/template.go
package notification

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
)

type templateEngine struct {
	templates map[string]*template.Template
}

// TemplateOption 配置模板引擎。
type TemplateOption func(*templateEngine)

// WithTemplateDir 从文件目录加载模板。
func WithTemplateDir(dir string) TemplateOption {
	return func(e *templateEngine) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			tmpl, err := template.New(name).Parse(string(data))
			if err != nil {
				e.templates[name] = nil // 标记存在但解析失败
				continue
			}
			e.templates[name] = tmpl
		}
	}
}

// WithTemplateFS 从 embed.FS 或任意 fs.FS 加载模板。
func WithTemplateFS(fsys fs.FS, root string) TemplateOption {
	return func(e *templateEngine) {
		sub, err := fs.Sub(fsys, root)
		if err != nil {
			return
		}
		fs.WalkDir(sub, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := fs.ReadFile(sub, path)
			if err != nil {
				return nil
			}
			tmpl, err := template.New(path).Parse(string(data))
			if err != nil {
				e.templates[path] = nil
				return nil
			}
			e.templates[path] = tmpl
			return nil
		})
	}
}

// NewTemplateEngine 创建模板引擎。
func NewTemplateEngine(opts ...TemplateOption) *templateEngine {
	e := &templateEngine{templates: make(map[string]*template.Template)}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Render 渲染指定模板。
func (e *templateEngine) Render(templateID string, data map[string]any) (string, error) {
	tmpl, ok := e.templates[templateID]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrTemplateNotFound, templateID)
	}
	if tmpl == nil {
		return "", fmt.Errorf("%w: %s (解析失败)", ErrTemplateRender, templateID)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTemplateRender, err)
	}
	return buf.String(), nil
}
```

### Test Command
```bash
go test ./notification/ -run TestTemplateEngine -v
```

### Commit Message
```
feat(notification): TemplateEngine 接口与 html/template 内置实现
```

---

## Task 3: Dispatcher -- `options.go` + `dispatcher.go`

### Files to Create
- `notification/options.go`
- `notification/dispatcher.go`
- `notification/dispatcher_test.go`

### Test Code -- `notification/dispatcher_test.go`

```go
// notification/dispatcher_test.go
package notification

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type mockSender struct {
	channel  Channel
	sendFunc func(ctx context.Context, msg *Message) (*Result, error)
	closed   bool
}

func newMockSender(ch Channel) *mockSender {
	return &mockSender{
		channel: ch,
		sendFunc: func(_ context.Context, _ *Message) (*Result, error) {
			return &Result{MessageID: "mock-id", Channel: ch}, nil
		},
	}
}

func (m *mockSender) Send(ctx context.Context, msg *Message) (*Result, error) {
	return m.sendFunc(ctx, msg)
}
func (m *mockSender) Channel() Channel { return m.channel }
func (m *mockSender) Close() error     { m.closed = true; return nil }

func TestDispatcher_Send(t *testing.T) {
	d := NewDispatcher()
	d.Register(newMockSender(ChannelEmail))

	result, err := d.Send(context.Background(), &Message{Channel: ChannelEmail, To: []string{"a@b.com"}, Body: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if result.MessageID != "mock-id" {
		t.Errorf("messageID = %q", result.MessageID)
	}
}

func TestDispatcher_Send_NoSender(t *testing.T) {
	d := NewDispatcher()
	_, err := d.Send(context.Background(), &Message{Channel: ChannelEmail, To: []string{"a@b.com"}, Body: "hi"})
	if !errors.Is(err, ErrNoSender) {
		t.Errorf("got %v, want ErrNoSender", err)
	}
}

func TestDispatcher_Send_InvalidMessage(t *testing.T) {
	d := NewDispatcher()
	_, err := d.Send(context.Background(), nil)
	if !errors.Is(err, ErrNilMessage) {
		t.Errorf("got %v, want ErrNilMessage", err)
	}
}

func TestDispatcher_Send_WithDefaultChannel(t *testing.T) {
	d := NewDispatcher(WithDefaultChannel(ChannelSMS))
	d.Register(newMockSender(ChannelSMS))

	result, err := d.Send(context.Background(), &Message{To: []string{"13800138000"}, Body: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Channel != ChannelSMS {
		t.Errorf("channel = %q, want sms", result.Channel)
	}
}

func TestDispatcher_Send_WithTemplateEngine(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "welcome.html"), []byte(`Hello, {{.Name}}!`), 0o644)

	eng := NewTemplateEngine(WithTemplateDir(dir))
	d := NewDispatcher(WithTemplateEngine(eng))
	s := newMockSender(ChannelEmail)
	var capturedBody string
	s.sendFunc = func(_ context.Context, msg *Message) (*Result, error) {
		capturedBody = msg.Body
		return &Result{MessageID: "t-1", Channel: ChannelEmail}, nil
	}
	d.Register(s)

	_, err := d.Send(context.Background(), &Message{
		Channel: ChannelEmail, To: []string{"a@b.com"},
		TemplateID: "welcome.html", TemplateData: map[string]any{"Name": "Alice"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if capturedBody != "Hello, Alice!" {
		t.Errorf("body = %q", capturedBody)
	}
}

func TestDispatcher_Broadcast(t *testing.T) {
	d := NewDispatcher()
	d.Register(newMockSender(ChannelEmail))
	d.Register(newMockSender(ChannelSMS))

	results := d.Broadcast(context.Background(), []Channel{ChannelEmail, ChannelSMS}, &Message{To: []string{"user"}, Body: "alert"})
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
}

func TestDispatcher_Broadcast_PartialFailure(t *testing.T) {
	d := NewDispatcher()
	d.Register(newMockSender(ChannelEmail))
	smsSender := newMockSender(ChannelSMS)
	smsSender.sendFunc = func(_ context.Context, _ *Message) (*Result, error) {
		return nil, errors.New("sms failed")
	}
	d.Register(smsSender)

	results := d.Broadcast(context.Background(), []Channel{ChannelEmail, ChannelSMS}, &Message{To: []string{"user"}, Body: "alert"})
	if results[0].Error != nil {
		t.Errorf("email should succeed: %v", results[0].Error)
	}
	if results[1].Error == nil {
		t.Error("sms should fail")
	}
}

func TestDispatcher_Close(t *testing.T) {
	d := NewDispatcher()
	s := newMockSender(ChannelEmail)
	d.Register(s)

	d.Close()
	if !s.closed {
		t.Error("sender should be closed")
	}
	_, err := d.Send(context.Background(), &Message{Channel: ChannelEmail, To: []string{"a@b.com"}, Body: "hi"})
	if !errors.Is(err, ErrClosed) {
		t.Errorf("got %v, want ErrClosed", err)
	}
}
```

### Implementation Code -- `notification/options.go`

```go
// notification/options.go
package notification

import (
	"github.com/Tsukikage7/servex/jobqueue"
	"github.com/Tsukikage7/servex/logger"
)

type dispatcherOptions struct {
	logger         logger.Logger
	templateEngine TemplateEngine
	jobClient      jobqueue.Client
	defaultChannel Channel
}

// Option 配置 Dispatcher。
type Option func(*dispatcherOptions)

// WithLogger 设置日志器。
func WithLogger(log logger.Logger) Option {
	return func(o *dispatcherOptions) { o.logger = log }
}

// WithTemplateEngine 设置模板引擎。
func WithTemplateEngine(eng TemplateEngine) Option {
	return func(o *dispatcherOptions) { o.templateEngine = eng }
}

// WithJobQueue 设置异步任务队列客户端。
func WithJobQueue(client jobqueue.Client) Option {
	return func(o *dispatcherOptions) { o.jobClient = client }
}

// WithDefaultChannel 设置默认发送渠道。
func WithDefaultChannel(ch Channel) Option {
	return func(o *dispatcherOptions) { o.defaultChannel = ch }
}
```

### Implementation Code -- `notification/dispatcher.go`

```go
// notification/dispatcher.go
package notification

import (
	"context"
	"sync"
	"sync/atomic"
)

// Dispatcher 消息调度器，根据 Channel 路由到对应 Sender。
type Dispatcher struct {
	opts    dispatcherOptions
	senders map[Channel]Sender
	mu      sync.RWMutex
	closed  atomic.Bool
}

// NewDispatcher 创建消息调度器。
func NewDispatcher(opts ...Option) *Dispatcher {
	var o dispatcherOptions
	for _, opt := range opts {
		opt(&o)
	}
	return &Dispatcher{opts: o, senders: make(map[Channel]Sender)}
}

// Register 注册一个 Sender。
func (d *Dispatcher) Register(sender Sender) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.senders[sender.Channel()] = sender
}

// Send 发送消息到消息指定的渠道。
func (d *Dispatcher) Send(ctx context.Context, msg *Message) (*Result, error) {
	if d.closed.Load() {
		return nil, ErrClosed
	}
	if msg != nil && msg.Channel == "" && d.opts.defaultChannel != "" {
		msg.Channel = d.opts.defaultChannel
	}
	if err := ValidateMessage(msg); err != nil {
		return nil, err
	}

	// 模板渲染
	if msg.TemplateID != "" && d.opts.templateEngine != nil {
		rendered, err := d.opts.templateEngine.Render(msg.TemplateID, msg.TemplateData)
		if err != nil {
			return nil, err
		}
		msg.Body = rendered
	}

	d.mu.RLock()
	sender, ok := d.senders[msg.Channel]
	d.mu.RUnlock()

	if !ok {
		return nil, ErrNoSender
	}
	return sender.Send(ctx, msg)
}

// Broadcast 向多个渠道广播同一消息。
func (d *Dispatcher) Broadcast(ctx context.Context, channels []Channel, msg *Message) []*Result {
	results := make([]*Result, 0, len(channels))
	for _, ch := range channels {
		clone := *msg
		clone.Channel = ch
		result, err := d.Send(ctx, &clone)
		if err != nil {
			results = append(results, &Result{Channel: ch, Error: err})
		} else {
			results = append(results, result)
		}
	}
	return results
}

// Close 关闭所有已注册的 Sender。
func (d *Dispatcher) Close() error {
	if d.closed.Swap(true) {
		return nil
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	var firstErr error
	for _, sender := range d.senders {
		if err := sender.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
```

### Test Command
```bash
go test ./notification/ -run TestDispatcher -v
```

### Commit Message
```
feat(notification): Dispatcher 调度器与 Option 函数
```

---

## Task 4: Email Sender -- `email/options.go` + `email/sender.go`

### Files to Create
- `notification/email/options.go`
- `notification/email/sender.go`
- `notification/email/sender_test.go`

### Test Code -- `notification/email/sender_test.go`

```go
// notification/email/sender_test.go
package email

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/Tsukikage7/servex/notification"
)

func TestSender_ImplementsInterface(t *testing.T) {
	var _ notification.Sender = (*Sender)(nil)
}

func TestSender_Channel(t *testing.T) {
	s, _ := NewSender(WithSMTP("localhost", 25), WithFrom("a@b.com", "Test"))
	if s.Channel() != notification.ChannelEmail {
		t.Errorf("channel = %q", s.Channel())
	}
}

func TestNewSender_MissingHost(t *testing.T) {
	_, err := NewSender(WithFrom("a@b.com", "Test"))
	if err == nil {
		t.Error("expected error for missing SMTP host")
	}
}

func TestNewSender_MissingFrom(t *testing.T) {
	_, err := NewSender(WithSMTP("localhost", 25))
	if err == nil {
		t.Error("expected error for missing from address")
	}
}

func TestSender_Send_ValidMessage(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	addr := ln.Addr().(*net.TCPAddr)
	go serveMockSMTP(t, ln)

	s, err := NewSender(WithSMTP("127.0.0.1", addr.Port), WithFrom("sender@test.com", "Test"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	msg := &notification.Message{
		Channel: notification.ChannelEmail,
		To:      []string{"recipient@test.com"},
		Subject: "Test Subject",
		Body:    "<h1>Hello</h1>",
	}
	result, err := s.Send(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if result.MessageID == "" {
		t.Error("expected non-empty message ID")
	}
}

func TestSender_Send_WithCCBCC(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	addr := ln.Addr().(*net.TCPAddr)
	go serveMockSMTP(t, ln)

	s, err := NewSender(WithSMTP("127.0.0.1", addr.Port), WithFrom("sender@test.com", "Test"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	msg := &notification.Message{
		Channel: notification.ChannelEmail, To: []string{"to@test.com"},
		Subject: "CC Test", Body: "body",
		Metadata: map[string]string{"cc": "cc1@test.com,cc2@test.com", "bcc": "bcc@test.com"},
	}
	_, err = s.Send(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSender_Send_NilMessage(t *testing.T) {
	s, _ := NewSender(WithSMTP("localhost", 25), WithFrom("a@b.com", "Test"))
	_, err := s.Send(context.Background(), nil)
	if !errors.Is(err, notification.ErrNilMessage) {
		t.Errorf("got %v, want ErrNilMessage", err)
	}
}

func TestSender_Close_Idempotent(t *testing.T) {
	s, _ := NewSender(WithSMTP("localhost", 25), WithFrom("a@b.com", "Test"))
	s.Close()
	if err := s.Close(); err != nil {
		t.Fatal("second close should not error")
	}
}

// serveMockSMTP 简易 SMTP mock 服务。
func serveMockSMTP(t *testing.T, ln net.Listener) {
	t.Helper()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			c.Write([]byte("220 mock SMTP\r\n"))
			buf := make([]byte, 4096)
			for {
				n, err := c.Read(buf)
				if err != nil {
					return
				}
				line := string(buf[:n])
				switch {
				case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
					c.Write([]byte("250 OK\r\n"))
				case strings.HasPrefix(line, "MAIL FROM"):
					c.Write([]byte("250 OK\r\n"))
				case strings.HasPrefix(line, "RCPT TO"):
					c.Write([]byte("250 OK\r\n"))
				case strings.HasPrefix(line, "DATA"):
					c.Write([]byte("354 Go ahead\r\n"))
				case strings.HasSuffix(strings.TrimSpace(line), "."):
					c.Write([]byte("250 OK\r\n"))
				case strings.HasPrefix(line, "QUIT"):
					c.Write([]byte("221 Bye\r\n"))
					return
				default:
					c.Write([]byte("250 OK\r\n"))
				}
			}
		}(conn)
	}
}
```

### Implementation Code -- `notification/email/options.go`

```go
// notification/email/options.go
package email

import "github.com/Tsukikage7/servex/logger"

type senderOptions struct {
	host     string
	port     int
	username string
	password string
	fromAddr string
	fromName string
	useTLS   bool
	logger   logger.Logger
}

type Option func(*senderOptions)

func WithSMTP(host string, port int) Option {
	return func(o *senderOptions) { o.host = host; o.port = port }
}

func WithAuth(username, password string) Option {
	return func(o *senderOptions) { o.username = username; o.password = password }
}

func WithFrom(addr, name string) Option {
	return func(o *senderOptions) { o.fromAddr = addr; o.fromName = name }
}

func WithTLS(enable bool) Option {
	return func(o *senderOptions) { o.useTLS = enable }
}

func WithLogger(log logger.Logger) Option {
	return func(o *senderOptions) { o.logger = log }
}
```

### Implementation Code -- `notification/email/sender.go`

```go
// notification/email/sender.go
package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/Tsukikage7/servex/notification"
)

type Sender struct {
	opts   senderOptions
	closed atomic.Bool
}

func NewSender(opts ...Option) (*Sender, error) {
	var o senderOptions
	for _, opt := range opts {
		opt(&o)
	}
	if o.host == "" {
		return nil, errors.New("notification/email: SMTP host 不能为空")
	}
	if o.fromAddr == "" {
		return nil, errors.New("notification/email: 发件人地址不能为空")
	}
	return &Sender{opts: o}, nil
}

func (s *Sender) Channel() notification.Channel { return notification.ChannelEmail }

func (s *Sender) Send(ctx context.Context, msg *notification.Message) (*notification.Result, error) {
	if msg == nil {
		return nil, notification.ErrNilMessage
	}
	if s.closed.Load() {
		return nil, notification.ErrClosed
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
	return &notification.Result{MessageID: msgID, Channel: notification.ChannelEmail}, nil
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

func (s *Sender) Close() error { s.closed.Store(true); return nil }
```

### Test Command
```bash
go test ./notification/email/ -v
```

### Commit Message
```
feat(notification): Email SMTP Sender 实现
```

---

## Task 5: SMS Sender -- `sms/provider.go` + `sms/options.go` + `sms/sender.go`

### Files to Create
- `notification/sms/provider.go`
- `notification/sms/options.go`
- `notification/sms/sender.go`
- `notification/sms/sender_test.go`

### Test Code -- `notification/sms/sender_test.go`

```go
// notification/sms/sender_test.go
package sms

import (
	"context"
	"errors"
	"testing"

	"github.com/Tsukikage7/servex/notification"
)

type mockProvider struct {
	name     string
	sendFunc func(ctx context.Context, req *SendRequest) (string, error)
}

func (m *mockProvider) Send(ctx context.Context, req *SendRequest) (string, error) {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, req)
	}
	return "mock-msg-id", nil
}
func (m *mockProvider) Name() string { return m.name }

func TestSender_ImplementsInterface(t *testing.T) {
	var _ notification.Sender = (*Sender)(nil)
}

func TestSender_Channel(t *testing.T) {
	s, _ := NewSender(&mockProvider{name: "mock"})
	if s.Channel() != notification.ChannelSMS {
		t.Errorf("channel = %q", s.Channel())
	}
}

func TestNewSender_NilProvider(t *testing.T) {
	_, err := NewSender(nil)
	if err == nil {
		t.Error("expected error for nil provider")
	}
}

func TestSender_Send(t *testing.T) {
	s, _ := NewSender(&mockProvider{name: "mock"}, WithSignName("MyApp"))
	result, err := s.Send(context.Background(), &notification.Message{
		Channel: notification.ChannelSMS, To: []string{"13800138000"}, Body: "验证码：1234",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.MessageID != "mock-msg-id" {
		t.Errorf("messageID = %q", result.MessageID)
	}
}

func TestSender_Send_WithTemplate(t *testing.T) {
	var captured *SendRequest
	p := &mockProvider{name: "mock", sendFunc: func(_ context.Context, req *SendRequest) (string, error) {
		captured = req
		return "t-1", nil
	}}
	s, _ := NewSender(p, WithSignName("App"))
	_, err := s.Send(context.Background(), &notification.Message{
		Channel: notification.ChannelSMS, To: []string{"13800138000"},
		TemplateID: "SMS_001", TemplateData: map[string]any{"code": "9999"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if captured.TemplateCode != "SMS_001" {
		t.Errorf("templateCode = %q", captured.TemplateCode)
	}
	if captured.Params["code"] != "9999" {
		t.Errorf("params = %v", captured.Params)
	}
}

func TestSender_Send_MultipleRecipients(t *testing.T) {
	callCount := 0
	p := &mockProvider{name: "mock", sendFunc: func(_ context.Context, _ *SendRequest) (string, error) {
		callCount++
		return "id", nil
	}}
	s, _ := NewSender(p)
	_, err := s.Send(context.Background(), &notification.Message{
		Channel: notification.ChannelSMS, To: []string{"1", "2", "3"}, Body: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if callCount != 3 {
		t.Errorf("provider called %d times, want 3", callCount)
	}
}

func TestSender_Send_ProviderError(t *testing.T) {
	p := &mockProvider{name: "mock", sendFunc: func(_ context.Context, _ *SendRequest) (string, error) {
		return "", errors.New("provider error")
	}}
	s, _ := NewSender(p)
	_, err := s.Send(context.Background(), &notification.Message{Channel: notification.ChannelSMS, To: []string{"1"}, Body: "x"})
	if err == nil {
		t.Error("expected error")
	}
}

func TestSender_Send_NilMessage(t *testing.T) {
	s, _ := NewSender(&mockProvider{name: "mock"})
	_, err := s.Send(context.Background(), nil)
	if !errors.Is(err, notification.ErrNilMessage) {
		t.Errorf("got %v, want ErrNilMessage", err)
	}
}
```

### Implementation Code -- `notification/sms/provider.go`

```go
// notification/sms/provider.go
package sms

import "context"

// Provider 短信服务商接口。
type Provider interface {
	Send(ctx context.Context, req *SendRequest) (messageID string, err error)
	Name() string
}

// SendRequest 短信发送请求。
type SendRequest struct {
	Phone        string
	Content      string
	SignName     string
	TemplateCode string
	Params       map[string]string
}
```

### Implementation Code -- `notification/sms/options.go`

```go
// notification/sms/options.go
package sms

import "github.com/Tsukikage7/servex/logger"

type senderOptions struct {
	signName string
	logger   logger.Logger
}

type Option func(*senderOptions)

func WithSignName(name string) Option {
	return func(o *senderOptions) { o.signName = name }
}

func WithLogger(log logger.Logger) Option {
	return func(o *senderOptions) { o.logger = log }
}
```

### Implementation Code -- `notification/sms/sender.go`

```go
// notification/sms/sender.go
package sms

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/Tsukikage7/servex/notification"
)

type Sender struct {
	provider Provider
	opts     senderOptions
	closed   atomic.Bool
}

func NewSender(provider Provider, opts ...Option) (*Sender, error) {
	if provider == nil {
		return nil, errors.New("notification/sms: provider 不能为空")
	}
	var o senderOptions
	for _, opt := range opts {
		opt(&o)
	}
	return &Sender{provider: provider, opts: o}, nil
}

func (s *Sender) Channel() notification.Channel { return notification.ChannelSMS }

func (s *Sender) Send(ctx context.Context, msg *notification.Message) (*notification.Result, error) {
	if msg == nil {
		return nil, notification.ErrNilMessage
	}
	if s.closed.Load() {
		return nil, notification.ErrClosed
	}

	params := make(map[string]string, len(msg.TemplateData))
	for k, v := range msg.TemplateData {
		params[k] = fmt.Sprintf("%v", v)
	}

	var lastID string
	var errs []string
	for _, phone := range msg.To {
		id, err := s.provider.Send(ctx, &SendRequest{
			Phone: phone, Content: msg.Body, SignName: s.opts.signName,
			TemplateCode: msg.TemplateID, Params: params,
		})
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", phone, err))
			continue
		}
		lastID = id
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("notification/sms: 部分发送失败: %s", strings.Join(errs, "; "))
	}
	return &notification.Result{MessageID: lastID, Channel: notification.ChannelSMS}, nil
}

func (s *Sender) Close() error { s.closed.Store(true); return nil }
```

### Test Command
```bash
go test ./notification/sms/ -run TestSender -v
```

### Commit Message
```
feat(notification): SMS Sender 与 Provider 接口
```

---

## Task 6: SMS Providers -- `sms/aliyun.go` + `sms/tencent.go`

### Files to Create
- `notification/sms/aliyun.go`
- `notification/sms/tencent.go`
- `notification/sms/provider_test.go`

### Test Code -- `notification/sms/provider_test.go`

```go
// notification/sms/provider_test.go
package sms

import (
	"context"
	"testing"
)

func TestAliyunProvider_ImplementsInterface(t *testing.T) { var _ Provider = (*AliyunProvider)(nil) }

func TestAliyunProvider_Name(t *testing.T) {
	p := NewAliyunProvider(AliyunConfig{AccessKeyID: "ak", AccessKeySecret: "sk", SignName: "Test"})
	if p.Name() != "aliyun" {
		t.Errorf("name = %q", p.Name())
	}
}

func TestAliyunProvider_Send_Stub(t *testing.T) {
	p := NewAliyunProvider(AliyunConfig{AccessKeyID: "ak", AccessKeySecret: "sk"})
	id, err := p.Send(context.Background(), &SendRequest{Phone: "13800138000", TemplateCode: "SMS_001"})
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Error("expected non-empty stub message ID")
	}
}

func TestTencentProvider_ImplementsInterface(t *testing.T) { var _ Provider = (*TencentProvider)(nil) }

func TestTencentProvider_Name(t *testing.T) {
	p := NewTencentProvider(TencentConfig{SecretID: "sid", SecretKey: "skey", AppID: "app"})
	if p.Name() != "tencent" {
		t.Errorf("name = %q", p.Name())
	}
}

func TestTencentProvider_Send_Stub(t *testing.T) {
	p := NewTencentProvider(TencentConfig{SecretID: "sid", SecretKey: "skey", AppID: "app"})
	id, err := p.Send(context.Background(), &SendRequest{Phone: "13800138000", TemplateCode: "T_001"})
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Error("expected non-empty stub message ID")
	}
}
```

### Implementation Code -- `notification/sms/aliyun.go`

```go
// notification/sms/aliyun.go
package sms

import (
	"context"
	"github.com/google/uuid"
)

type AliyunConfig struct {
	AccessKeyID     string `json:"access_key_id"     yaml:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret" yaml:"access_key_secret"`
	SignName        string `json:"sign_name"         yaml:"sign_name"`
	Endpoint        string `json:"endpoint"          yaml:"endpoint"`
}

type AliyunProvider struct{ config AliyunConfig }

func NewAliyunProvider(cfg AliyunConfig) *AliyunProvider { return &AliyunProvider{config: cfg} }
func (p *AliyunProvider) Name() string                   { return "aliyun" }

// Send 桩实现。TODO: 接入阿里云 SMS SDK。
func (p *AliyunProvider) Send(_ context.Context, _ *SendRequest) (string, error) {
	return "aliyun-stub-" + uuid.New().String(), nil
}
```

### Implementation Code -- `notification/sms/tencent.go`

```go
// notification/sms/tencent.go
package sms

import (
	"context"
	"github.com/google/uuid"
)

type TencentConfig struct {
	SecretID  string `json:"secret_id"  yaml:"secret_id"`
	SecretKey string `json:"secret_key" yaml:"secret_key"`
	AppID     string `json:"app_id"     yaml:"app_id"`
	SignName  string `json:"sign_name"  yaml:"sign_name"`
	Endpoint  string `json:"endpoint"   yaml:"endpoint"`
}

type TencentProvider struct{ config TencentConfig }

func NewTencentProvider(cfg TencentConfig) *TencentProvider { return &TencentProvider{config: cfg} }
func (p *TencentProvider) Name() string                     { return "tencent" }

// Send 桩实现。TODO: 接入腾讯云 SMS SDK。
func (p *TencentProvider) Send(_ context.Context, _ *SendRequest) (string, error) {
	return "tencent-stub-" + uuid.New().String(), nil
}
```

### Test Command
```bash
go test ./notification/sms/ -run "TestAliyun|TestTencent" -v
```

### Commit Message
```
feat(notification): SMS 阿里云/腾讯云 Provider 桩实现
```

---

## Task 7: Webhook Sender -- `webhook/format.go` + `webhook/options.go` + `webhook/sender.go`

### Files to Create
- `notification/webhook/format.go`
- `notification/webhook/options.go`
- `notification/webhook/sender.go`
- `notification/webhook/sender_test.go`

### Test Code -- `notification/webhook/sender_test.go`

```go
// notification/webhook/sender_test.go
package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tsukikage7/servex/notification"
)

func TestSender_ImplementsInterface(t *testing.T) { var _ notification.Sender = (*Sender)(nil) }

func TestSender_Channel(t *testing.T) {
	s, _ := NewSender()
	if s.Channel() != notification.ChannelWebhook {
		t.Errorf("channel = %q", s.Channel())
	}
}

func TestSender_Send_Custom(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s, _ := NewSender()
	defer s.Close()
	msg := &notification.Message{
		Channel: notification.ChannelWebhook, To: []string{server.URL},
		Body: `{"text":"hello"}`, Metadata: map[string]string{"format": "custom"},
	}
	result, err := s.Send(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if result.MessageID == "" {
		t.Error("expected non-empty message ID")
	}
	if string(receivedBody) != `{"text":"hello"}` {
		t.Errorf("body = %s", receivedBody)
	}
}

func TestSender_Send_SlackFormat(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s, _ := NewSender()
	_, err := s.Send(context.Background(), &notification.Message{
		Channel: notification.ChannelWebhook, To: []string{server.URL},
		Subject: "Alert", Body: "Server down", Metadata: map[string]string{"format": "slack"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	json.Unmarshal(receivedBody, &payload)
	if payload["text"] == nil {
		t.Error("slack payload should have 'text' field")
	}
}

func TestSender_Send_WithHMAC(t *testing.T) {
	var receivedSig string
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Signature")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s, _ := NewSender()
	_, err := s.Send(context.Background(), &notification.Message{
		Channel: notification.ChannelWebhook, To: []string{server.URL},
		Body: `{"event":"test"}`, Metadata: map[string]string{"secret": "my-secret", "format": "custom"},
	})
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, []byte("my-secret"))
	mac.Write(receivedBody)
	if expected := hex.EncodeToString(mac.Sum(nil)); receivedSig != expected {
		t.Errorf("sig mismatch: got %q, want %q", receivedSig, expected)
	}
}

func TestSender_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	s, _ := NewSender()
	_, err := s.Send(context.Background(), &notification.Message{
		Channel: notification.ChannelWebhook, To: []string{server.URL}, Body: "test",
	})
	if err == nil {
		t.Error("expected error for 500")
	}
}

func TestSender_Send_WithRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s, _ := NewSender(WithRetry(3))
	_, err := s.Send(context.Background(), &notification.Message{
		Channel: notification.ChannelWebhook, To: []string{server.URL}, Body: "retry",
	})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestSender_Send_NilMessage(t *testing.T) {
	s, _ := NewSender()
	_, err := s.Send(context.Background(), nil)
	if !errors.Is(err, notification.ErrNilMessage) {
		t.Errorf("got %v, want ErrNilMessage", err)
	}
}

func TestFormatters(t *testing.T) {
	for _, format := range []string{"slack", "dingtalk", "lark"} {
		t.Run(format, func(t *testing.T) {
			f := getFormatter(format)
			data := f("Title", "Body")
			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				t.Errorf("invalid JSON: %v, data=%s", err, data)
			}
		})
	}
}
```

### Implementation Code -- `notification/webhook/format.go`

```go
// notification/webhook/format.go
package webhook

import "encoding/json"

type Formatter func(subject, body string) []byte

func getFormatter(format string) Formatter {
	switch format {
	case "slack":
		return formatSlack
	case "dingtalk":
		return formatDingTalk
	case "lark":
		return formatLark
	default:
		return formatCustom
	}
}

func formatSlack(subject, body string) []byte {
	data, _ := json.Marshal(map[string]any{"text": subject + "\n" + body})
	return data
}

func formatDingTalk(subject, body string) []byte {
	data, _ := json.Marshal(map[string]any{
		"msgtype": "text", "text": map[string]string{"content": subject + "\n" + body},
	})
	return data
}

func formatLark(subject, body string) []byte {
	data, _ := json.Marshal(map[string]any{
		"msg_type": "text", "content": map[string]string{"text": subject + "\n" + body},
	})
	return data
}

func formatCustom(_, body string) []byte { return []byte(body) }
```

### Implementation Code -- `notification/webhook/options.go`

```go
// notification/webhook/options.go
package webhook

import (
	"net/http"
	"time"
	"github.com/Tsukikage7/servex/logger"
)

type senderOptions struct {
	httpClient *http.Client
	timeout    time.Duration
	maxRetry   int
	logger     logger.Logger
}

type Option func(*senderOptions)

func WithTimeout(d time.Duration) Option    { return func(o *senderOptions) { o.timeout = d } }
func WithRetry(n int) Option                { return func(o *senderOptions) { o.maxRetry = n } }
func WithHTTPClient(c *http.Client) Option  { return func(o *senderOptions) { o.httpClient = c } }
func WithLogger(log logger.Logger) Option   { return func(o *senderOptions) { o.logger = log } }
```

### Implementation Code -- `notification/webhook/sender.go`

```go
// notification/webhook/sender.go
package webhook

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
	"github.com/Tsukikage7/servex/notification"
)

type Sender struct {
	opts   senderOptions
	closed atomic.Bool
}

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

func (s *Sender) Channel() notification.Channel { return notification.ChannelWebhook }

func (s *Sender) Send(ctx context.Context, msg *notification.Message) (*notification.Result, error) {
	if msg == nil {
		return nil, notification.ErrNilMessage
	}
	if s.closed.Load() {
		return nil, notification.ErrClosed
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
			return &notification.Result{MessageID: msgID, Channel: notification.ChannelWebhook}, nil
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

func (s *Sender) Close() error { s.closed.Store(true); return nil }
```

### Test Command
```bash
go test ./notification/webhook/ -v
```

### Commit Message
```
feat(notification): Webhook Sender 与 Slack/DingTalk/Lark 格式化器
```

---

## Task 8: Push Sender -- `push/provider.go` + `push/options.go` + `push/sender.go`

### Files to Create
- `notification/push/provider.go`
- `notification/push/options.go`
- `notification/push/sender.go`
- `notification/push/sender_test.go`

### Test Code -- `notification/push/sender_test.go`

```go
// notification/push/sender_test.go
package push

import (
	"context"
	"errors"
	"testing"
	"github.com/Tsukikage7/servex/notification"
)

type mockProvider struct {
	name     string
	sendFunc func(ctx context.Context, token string, payload *Payload) (string, error)
}

func (m *mockProvider) Send(ctx context.Context, token string, payload *Payload) (string, error) {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, token, payload)
	}
	return "mock-push-id", nil
}
func (m *mockProvider) Name() string { return m.name }

func TestSender_ImplementsInterface(t *testing.T) { var _ notification.Sender = (*Sender)(nil) }

func TestSender_Channel(t *testing.T) {
	s, _ := NewSender(&mockProvider{name: "mock"})
	if s.Channel() != notification.ChannelPush {
		t.Errorf("channel = %q", s.Channel())
	}
}

func TestNewSender_NilProvider(t *testing.T) {
	_, err := NewSender(nil)
	if err == nil {
		t.Error("expected error for nil provider")
	}
}

func TestSender_Send(t *testing.T) {
	s, _ := NewSender(&mockProvider{name: "mock"})
	result, err := s.Send(context.Background(), &notification.Message{
		Channel: notification.ChannelPush, To: []string{"device-token"},
		Subject: "Title", Body: "Body", Metadata: map[string]string{"badge": "5", "sound": "default"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.MessageID != "mock-push-id" {
		t.Errorf("messageID = %q", result.MessageID)
	}
}

func TestSender_Send_MultipleTokens(t *testing.T) {
	callCount := 0
	p := &mockProvider{name: "mock", sendFunc: func(_ context.Context, _ string, _ *Payload) (string, error) {
		callCount++
		return "id", nil
	}}
	s, _ := NewSender(p)
	_, err := s.Send(context.Background(), &notification.Message{
		Channel: notification.ChannelPush, To: []string{"t1", "t2"}, Body: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if callCount != 2 {
		t.Errorf("called %d times, want 2", callCount)
	}
}

func TestSender_Send_ProviderError(t *testing.T) {
	p := &mockProvider{name: "mock", sendFunc: func(_ context.Context, _ string, _ *Payload) (string, error) {
		return "", errors.New("push failed")
	}}
	s, _ := NewSender(p)
	_, err := s.Send(context.Background(), &notification.Message{
		Channel: notification.ChannelPush, To: []string{"t"}, Body: "test",
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestSender_Send_NilMessage(t *testing.T) {
	s, _ := NewSender(&mockProvider{name: "mock"})
	_, err := s.Send(context.Background(), nil)
	if !errors.Is(err, notification.ErrNilMessage) {
		t.Errorf("got %v, want ErrNilMessage", err)
	}
}
```

### Implementation Code -- `notification/push/provider.go`

```go
// notification/push/provider.go
package push

import "context"

type Provider interface {
	Send(ctx context.Context, token string, payload *Payload) (messageID string, err error)
	Name() string
}

type Payload struct {
	Title string
	Body  string
	Data  map[string]string
	Badge int
	Sound string
}
```

### Implementation Code -- `notification/push/options.go`

```go
// notification/push/options.go
package push

import "github.com/Tsukikage7/servex/logger"

type senderOptions struct{ logger logger.Logger }
type Option func(*senderOptions)

func WithLogger(log logger.Logger) Option { return func(o *senderOptions) { o.logger = log } }
```

### Implementation Code -- `notification/push/sender.go`

```go
// notification/push/sender.go
package push

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"github.com/Tsukikage7/servex/notification"
)

type Sender struct {
	provider Provider
	opts     senderOptions
	closed   atomic.Bool
}

func NewSender(provider Provider, opts ...Option) (*Sender, error) {
	if provider == nil {
		return nil, errors.New("notification/push: provider 不能为空")
	}
	var o senderOptions
	for _, opt := range opts {
		opt(&o)
	}
	return &Sender{provider: provider, opts: o}, nil
}

func (s *Sender) Channel() notification.Channel { return notification.ChannelPush }

func (s *Sender) Send(ctx context.Context, msg *notification.Message) (*notification.Result, error) {
	if msg == nil {
		return nil, notification.ErrNilMessage
	}
	if s.closed.Load() {
		return nil, notification.ErrClosed
	}

	payload := &Payload{Title: msg.Subject, Body: msg.Body}
	if msg.Metadata != nil {
		if b, ok := msg.Metadata["badge"]; ok {
			if n, err := strconv.Atoi(b); err == nil {
				payload.Badge = n
			}
		}
		payload.Sound = msg.Metadata["sound"]
	}
	if len(msg.TemplateData) > 0 {
		payload.Data = make(map[string]string, len(msg.TemplateData))
		for k, v := range msg.TemplateData {
			payload.Data[k] = fmt.Sprintf("%v", v)
		}
	}

	var lastID string
	var errs []string
	for _, token := range msg.To {
		id, err := s.provider.Send(ctx, token, payload)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", token, err))
			continue
		}
		lastID = id
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("notification/push: 部分发送失败: %s", strings.Join(errs, "; "))
	}
	return &notification.Result{MessageID: lastID, Channel: notification.ChannelPush}, nil
}

func (s *Sender) Close() error { s.closed.Store(true); return nil }
```

### Test Command
```bash
go test ./notification/push/ -run TestSender -v
```

### Commit Message
```
feat(notification): Push Sender 与 Provider 接口
```

---

## Task 9: Push Providers -- `push/fcm.go` + `push/apns.go`

### Files to Create
- `notification/push/fcm.go`
- `notification/push/apns.go`
- `notification/push/provider_test.go`

### Test Code -- `notification/push/provider_test.go`

```go
// notification/push/provider_test.go
package push

import (
	"context"
	"testing"
)

func TestFCMProvider_ImplementsInterface(t *testing.T)  { var _ Provider = (*FCMProvider)(nil) }
func TestAPNsProvider_ImplementsInterface(t *testing.T) { var _ Provider = (*APNsProvider)(nil) }

func TestFCMProvider_Name(t *testing.T) {
	p := NewFCMProvider(FCMConfig{ProjectID: "proj"})
	if p.Name() != "fcm" {
		t.Errorf("name = %q", p.Name())
	}
}

func TestFCMProvider_Send_Stub(t *testing.T) {
	p := NewFCMProvider(FCMConfig{ProjectID: "proj"})
	id, err := p.Send(context.Background(), "token", &Payload{Title: "T", Body: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Error("expected non-empty stub ID")
	}
}

func TestAPNsProvider_Name(t *testing.T) {
	p := NewAPNsProvider(APNsConfig{BundleID: "com.example.app"})
	if p.Name() != "apns" {
		t.Errorf("name = %q", p.Name())
	}
}

func TestAPNsProvider_Send_Stub(t *testing.T) {
	p := NewAPNsProvider(APNsConfig{BundleID: "com.example.app"})
	id, err := p.Send(context.Background(), "token", &Payload{Title: "T", Body: "B", Badge: 1, Sound: "default"})
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Error("expected non-empty stub ID")
	}
}
```

### Implementation Code -- `notification/push/fcm.go`

```go
// notification/push/fcm.go
package push

import (
	"context"
	"github.com/google/uuid"
)

type FCMConfig struct {
	ProjectID       string `json:"project_id"       yaml:"project_id"`
	CredentialsJSON []byte `json:"credentials_json" yaml:"credentials_json"`
}

type FCMProvider struct{ config FCMConfig }

func NewFCMProvider(cfg FCMConfig) *FCMProvider { return &FCMProvider{config: cfg} }
func (p *FCMProvider) Name() string             { return "fcm" }

// Send 桩实现。TODO: 接入 Firebase Admin SDK。
func (p *FCMProvider) Send(_ context.Context, _ string, _ *Payload) (string, error) {
	return "fcm-stub-" + uuid.New().String(), nil
}
```

### Implementation Code -- `notification/push/apns.go`

```go
// notification/push/apns.go
package push

import (
	"context"
	"github.com/google/uuid"
)

type APNsConfig struct {
	BundleID   string `json:"bundle_id"   yaml:"bundle_id"`
	TeamID     string `json:"team_id"     yaml:"team_id"`
	KeyID      string `json:"key_id"      yaml:"key_id"`
	KeyFile    string `json:"key_file"    yaml:"key_file"`
	Production bool   `json:"production"  yaml:"production"`
}

type APNsProvider struct{ config APNsConfig }

func NewAPNsProvider(cfg APNsConfig) *APNsProvider { return &APNsProvider{config: cfg} }
func (p *APNsProvider) Name() string               { return "apns" }

// Send 桩实现。TODO: 接入 Apple APNs HTTP/2 API。
func (p *APNsProvider) Send(_ context.Context, _ string, _ *Payload) (string, error) {
	return "apns-stub-" + uuid.New().String(), nil
}
```

### Test Command
```bash
go test ./notification/push/ -run "TestFCM|TestAPNs" -v
```

### Commit Message
```
feat(notification): Push FCM/APNs Provider 桩实现
```

---

## Task 10: Dispatcher Async -- SendAsync

### Files to Modify
- `notification/dispatcher.go` -- add `SendAsync` method
- `notification/dispatcher_test.go` -- add async tests

### Additional Test Code (append to `dispatcher_test.go`)

```go
// Add to imports: "github.com/Tsukikage7/servex/jobqueue"

type mockJobClient struct {
	jobs []*jobqueue.Job
}

func (m *mockJobClient) Enqueue(_ context.Context, job *jobqueue.Job) error {
	m.jobs = append(m.jobs, job)
	return nil
}
func (m *mockJobClient) Close() error { return nil }

func TestDispatcher_SendAsync(t *testing.T) {
	client := &mockJobClient{}
	d := NewDispatcher(WithJobQueue(client))
	d.Register(newMockSender(ChannelEmail))

	msg := &Message{Channel: ChannelEmail, To: []string{"a@b.com"}, Subject: "Async", Body: "hello"}
	if err := d.SendAsync(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if len(client.jobs) != 1 {
		t.Fatalf("got %d jobs, want 1", len(client.jobs))
	}
	job := client.jobs[0]
	if job.Queue != "notifications" {
		t.Errorf("queue = %q", job.Queue)
	}
	if job.Type != "notification.email" {
		t.Errorf("type = %q", job.Type)
	}
}

func TestDispatcher_SendAsync_NoJobQueue(t *testing.T) {
	d := NewDispatcher()
	err := d.SendAsync(context.Background(), &Message{Channel: ChannelEmail, To: []string{"a@b.com"}, Body: "hi"})
	if err == nil {
		t.Error("expected error when no job queue")
	}
}

func TestDispatcher_SendAsync_InvalidMessage(t *testing.T) {
	client := &mockJobClient{}
	d := NewDispatcher(WithJobQueue(client))
	err := d.SendAsync(context.Background(), nil)
	if !errors.Is(err, ErrNilMessage) {
		t.Errorf("got %v, want ErrNilMessage", err)
	}
}
```

### Additional Implementation Code (add to `dispatcher.go`)

Add imports: `"encoding/json"`, `"fmt"`, `"github.com/Tsukikage7/servex/jobqueue"`, `"github.com/google/uuid"`.

```go
// SendAsync 将消息序列化后投入 jobqueue 异步发送。
func (d *Dispatcher) SendAsync(ctx context.Context, msg *Message) error {
	if d.closed.Load() {
		return ErrClosed
	}
	if msg != nil && msg.Channel == "" && d.opts.defaultChannel != "" {
		msg.Channel = d.opts.defaultChannel
	}
	if err := ValidateMessage(msg); err != nil {
		return err
	}
	if d.opts.jobClient == nil {
		return fmt.Errorf("notification: jobqueue 未配置")
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("notification: 序列化消息失败: %w", err)
	}

	return d.opts.jobClient.Enqueue(ctx, &jobqueue.Job{
		ID:    uuid.New().String(),
		Queue: "notifications",
		Type:  "notification." + string(msg.Channel),
		Payload: payload,
	})
}
```

### Test Command
```bash
go test ./notification/ -run TestDispatcher_SendAsync -v
```

### Commit Message
```
feat(notification): Dispatcher SendAsync 异步发送集成 jobqueue
```

---

## Task 11: Factory -- `factory/factory.go`

### Files to Create
- `notification/factory/factory.go`
- `notification/factory/factory_test.go`

### Test Code -- `notification/factory/factory_test.go`

```go
// notification/factory/factory_test.go
package factory

import "testing"

func TestNewDispatcher_NilConfig(t *testing.T) {
	_, err := NewDispatcher(nil, nil)
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestNewDispatcher_EmptyConfig(t *testing.T) {
	d, err := NewDispatcher(&Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}

func TestNewDispatcher_WithEmail(t *testing.T) {
	d, err := NewDispatcher(&Config{
		DefaultChannel: "email",
		Email:          &EmailConfig{Host: "smtp.example.com", Port: 587, From: "noreply@example.com", Name: "Test"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}

func TestNewDispatcher_WithSMS_Aliyun(t *testing.T) {
	d, err := NewDispatcher(&Config{SMS: &SMSConfig{
		Provider: "aliyun", SignName: "Test",
		Aliyun: &AliyunSMSConfig{AccessKeyID: "ak", AccessKeySecret: "sk"},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}

func TestNewDispatcher_WithSMS_Tencent(t *testing.T) {
	d, err := NewDispatcher(&Config{SMS: &SMSConfig{
		Provider: "tencent", SignName: "Test",
		Tencent: &TencentSMSConfig{SecretID: "sid", SecretKey: "skey", AppID: "app"},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}

func TestNewDispatcher_WithSMS_UnknownProvider(t *testing.T) {
	_, err := NewDispatcher(&Config{SMS: &SMSConfig{Provider: "unknown"}}, nil)
	if err == nil {
		t.Error("expected error for unknown SMS provider")
	}
}

func TestNewDispatcher_WithWebhook(t *testing.T) {
	d, err := NewDispatcher(&Config{Webhook: &WebhookConfig{Timeout: 5, Retry: 3}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}

func TestNewDispatcher_WithPush_FCM(t *testing.T) {
	d, err := NewDispatcher(&Config{Push: &PushConfig{Provider: "fcm", FCM: &FCMPushConfig{ProjectID: "proj"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}

func TestNewDispatcher_WithPush_APNs(t *testing.T) {
	d, err := NewDispatcher(&Config{Push: &PushConfig{Provider: "apns", APNs: &APNsPushConfig{BundleID: "com.example.app"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}

func TestNewDispatcher_WithPush_UnknownProvider(t *testing.T) {
	_, err := NewDispatcher(&Config{Push: &PushConfig{Provider: "unknown"}}, nil)
	if err == nil {
		t.Error("expected error for unknown push provider")
	}
}

func TestNewDispatcher_AllChannels(t *testing.T) {
	d, err := NewDispatcher(&Config{
		DefaultChannel: "email",
		Email:          &EmailConfig{Host: "smtp.example.com", Port: 587, From: "no@example.com", Name: "T"},
		SMS:            &SMSConfig{Provider: "aliyun", SignName: "T", Aliyun: &AliyunSMSConfig{AccessKeyID: "ak", AccessKeySecret: "sk"}},
		Webhook:        &WebhookConfig{Timeout: 10, Retry: 2},
		Push:           &PushConfig{Provider: "fcm", FCM: &FCMPushConfig{ProjectID: "p"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}
```

### Implementation Code -- `notification/factory/factory.go`

```go
// Package factory 提供 Config 驱动的 notification.Dispatcher 工厂。
package factory

import (
	"errors"
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/logger"
	"github.com/Tsukikage7/servex/notification"
	"github.com/Tsukikage7/servex/notification/email"
	"github.com/Tsukikage7/servex/notification/push"
	"github.com/Tsukikage7/servex/notification/sms"
	"github.com/Tsukikage7/servex/notification/webhook"
)

type Config struct {
	DefaultChannel string         `json:"default_channel" yaml:"default_channel"`
	TemplateDir    string         `json:"template_dir"    yaml:"template_dir"`
	Email          *EmailConfig   `json:"email"           yaml:"email"`
	SMS            *SMSConfig     `json:"sms"             yaml:"sms"`
	Webhook        *WebhookConfig `json:"webhook"         yaml:"webhook"`
	Push           *PushConfig    `json:"push"            yaml:"push"`
}

type EmailConfig struct {
	Host, Username, Password, From, Name string
	Port                                 int
	TLS                                  bool
}

type SMSConfig struct {
	Provider string
	SignName string
	Aliyun   *AliyunSMSConfig
	Tencent  *TencentSMSConfig
}

type AliyunSMSConfig struct{ AccessKeyID, AccessKeySecret, Endpoint string }
type TencentSMSConfig struct{ SecretID, SecretKey, AppID, Endpoint string }

type WebhookConfig struct {
	Timeout int
	Retry   int
}

type PushConfig struct {
	Provider string
	FCM      *FCMPushConfig
	APNs     *APNsPushConfig
}

type FCMPushConfig struct {
	ProjectID       string
	CredentialsJSON string
}

type APNsPushConfig struct {
	BundleID, TeamID, KeyID, KeyFile string
	Production                       bool
}

var errNilConfig = errors.New("notification: config 不能为空")

func NewDispatcher(cfg *Config, log logger.Logger) (*notification.Dispatcher, error) {
	if cfg == nil {
		return nil, errNilConfig
	}

	var opts []notification.Option
	if log != nil {
		opts = append(opts, notification.WithLogger(log))
	}
	if cfg.DefaultChannel != "" {
		opts = append(opts, notification.WithDefaultChannel(notification.Channel(cfg.DefaultChannel)))
	}
	if cfg.TemplateDir != "" {
		opts = append(opts, notification.WithTemplateEngine(notification.NewTemplateEngine(notification.WithTemplateDir(cfg.TemplateDir))))
	}

	d := notification.NewDispatcher(opts...)

	if cfg.Email != nil {
		eo := []email.Option{email.WithSMTP(cfg.Email.Host, cfg.Email.Port), email.WithFrom(cfg.Email.From, cfg.Email.Name)}
		if cfg.Email.Username != "" {
			eo = append(eo, email.WithAuth(cfg.Email.Username, cfg.Email.Password))
		}
		if cfg.Email.TLS {
			eo = append(eo, email.WithTLS(true))
		}
		s, err := email.NewSender(eo...)
		if err != nil {
			return nil, fmt.Errorf("notification: email sender: %w", err)
		}
		d.Register(s)
	}

	if cfg.SMS != nil {
		s, err := buildSMS(cfg.SMS, log)
		if err != nil {
			return nil, err
		}
		d.Register(s)
	}

	if cfg.Webhook != nil {
		wo := []webhook.Option{}
		if cfg.Webhook.Timeout > 0 {
			wo = append(wo, webhook.WithTimeout(time.Duration(cfg.Webhook.Timeout)*time.Second))
		}
		if cfg.Webhook.Retry > 0 {
			wo = append(wo, webhook.WithRetry(cfg.Webhook.Retry))
		}
		s, err := webhook.NewSender(wo...)
		if err != nil {
			return nil, fmt.Errorf("notification: webhook sender: %w", err)
		}
		d.Register(s)
	}

	if cfg.Push != nil {
		s, err := buildPush(cfg.Push, log)
		if err != nil {
			return nil, err
		}
		d.Register(s)
	}

	return d, nil
}

func buildSMS(cfg *SMSConfig, log logger.Logger) (notification.Sender, error) {
	var p sms.Provider
	switch cfg.Provider {
	case "aliyun":
		if cfg.Aliyun == nil {
			cfg.Aliyun = &AliyunSMSConfig{}
		}
		p = sms.NewAliyunProvider(sms.AliyunConfig{
			AccessKeyID: cfg.Aliyun.AccessKeyID, AccessKeySecret: cfg.Aliyun.AccessKeySecret,
			SignName: cfg.SignName, Endpoint: cfg.Aliyun.Endpoint,
		})
	case "tencent":
		if cfg.Tencent == nil {
			cfg.Tencent = &TencentSMSConfig{}
		}
		p = sms.NewTencentProvider(sms.TencentConfig{
			SecretID: cfg.Tencent.SecretID, SecretKey: cfg.Tencent.SecretKey,
			AppID: cfg.Tencent.AppID, SignName: cfg.SignName, Endpoint: cfg.Tencent.Endpoint,
		})
	default:
		return nil, fmt.Errorf("notification: 不支持的 SMS provider %q", cfg.Provider)
	}
	opts := []sms.Option{sms.WithSignName(cfg.SignName)}
	if log != nil {
		opts = append(opts, sms.WithLogger(log))
	}
	return sms.NewSender(p, opts...)
}

func buildPush(cfg *PushConfig, log logger.Logger) (notification.Sender, error) {
	var p push.Provider
	switch cfg.Provider {
	case "fcm":
		if cfg.FCM == nil {
			cfg.FCM = &FCMPushConfig{}
		}
		p = push.NewFCMProvider(push.FCMConfig{ProjectID: cfg.FCM.ProjectID, CredentialsJSON: []byte(cfg.FCM.CredentialsJSON)})
	case "apns":
		if cfg.APNs == nil {
			cfg.APNs = &APNsPushConfig{}
		}
		p = push.NewAPNsProvider(push.APNsConfig{
			BundleID: cfg.APNs.BundleID, TeamID: cfg.APNs.TeamID,
			KeyID: cfg.APNs.KeyID, KeyFile: cfg.APNs.KeyFile, Production: cfg.APNs.Production,
		})
	default:
		return nil, fmt.Errorf("notification: 不支持的 push provider %q", cfg.Provider)
	}
	var opts []push.Option
	if log != nil {
		opts = append(opts, push.WithLogger(log))
	}
	return push.NewSender(p, opts...)
}
```

### Test Command
```bash
go test ./notification/factory/ -v
```

### Commit Message
```
feat(notification): Config 驱动工厂
```

---

## Task 12: Integration Tests

### Files to Create
- `notification/integration_test.go`

### Test Code -- `notification/integration_test.go`

```go
// notification/integration_test.go
package notification

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Tsukikage7/servex/jobqueue"
)

type recordingSender struct {
	channel  Channel
	mu       sync.Mutex
	messages []*Message
	closed   bool
}

func newRecordingSender(ch Channel) *recordingSender { return &recordingSender{channel: ch} }

func (r *recordingSender) Send(_ context.Context, msg *Message) (*Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = append(r.messages, msg)
	return &Result{MessageID: "rec-" + string(r.channel), Channel: r.channel}, nil
}
func (r *recordingSender) Channel() Channel { return r.channel }
func (r *recordingSender) Close() error     { r.closed = true; return nil }
func (r *recordingSender) count() int       { r.mu.Lock(); defer r.mu.Unlock(); return len(r.messages) }
func (r *recordingSender) last() *Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.messages) == 0 {
		return nil
	}
	return r.messages[len(r.messages)-1]
}

func TestIntegration_MultiSender(t *testing.T) {
	emailRec := newRecordingSender(ChannelEmail)
	smsRec := newRecordingSender(ChannelSMS)
	d := NewDispatcher()
	d.Register(emailRec)
	d.Register(smsRec)
	ctx := context.Background()

	d.Send(ctx, &Message{Channel: ChannelEmail, To: []string{"a@b.com"}, Body: "hi"})
	d.Send(ctx, &Message{Channel: ChannelSMS, To: []string{"138"}, Body: "code"})

	results := d.Broadcast(ctx, []Channel{ChannelEmail, ChannelSMS}, &Message{To: []string{"u"}, Body: "broadcast"})
	if len(results) != 2 {
		t.Fatalf("broadcast results = %d", len(results))
	}
	if emailRec.count() != 2 {
		t.Errorf("email = %d, want 2", emailRec.count())
	}
	if smsRec.count() != 2 {
		t.Errorf("sms = %d, want 2", smsRec.count())
	}
}

func TestIntegration_WithTemplate(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "order.html"), []byte(`Order #{{.ID}} confirmed.`), 0o644)
	eng := NewTemplateEngine(WithTemplateDir(dir))
	rec := newRecordingSender(ChannelEmail)
	d := NewDispatcher(WithTemplateEngine(eng))
	d.Register(rec)

	d.Send(context.Background(), &Message{
		Channel: ChannelEmail, To: []string{"u@x.com"},
		TemplateID: "order.html", TemplateData: map[string]any{"ID": "12345"},
	})
	if msg := rec.last(); msg.Body != "Order #12345 confirmed." {
		t.Errorf("body = %q", msg.Body)
	}
}

func TestIntegration_AsyncRoundTrip(t *testing.T) {
	jc := &integrationJobClient{}
	rec := newRecordingSender(ChannelEmail)
	d := NewDispatcher(WithJobQueue(jc))
	d.Register(rec)

	msg := &Message{Channel: ChannelEmail, To: []string{"a@b.com"}, Body: "async"}
	d.SendAsync(context.Background(), msg)

	// Simulate consumer
	var decoded Message
	json.Unmarshal(jc.jobs[0].Payload, &decoded)
	d.Send(context.Background(), &decoded)

	if rec.count() != 1 {
		t.Errorf("count = %d, want 1", rec.count())
	}
}

func TestIntegration_CloseAll(t *testing.T) {
	e := newRecordingSender(ChannelEmail)
	s := newRecordingSender(ChannelSMS)
	w := newRecordingSender(ChannelWebhook)
	d := NewDispatcher()
	d.Register(e)
	d.Register(s)
	d.Register(w)
	d.Close()
	if !e.closed || !s.closed || !w.closed {
		t.Error("all senders should be closed")
	}
}

type integrationJobClient struct{ jobs []*jobqueue.Job }

func (c *integrationJobClient) Enqueue(_ context.Context, job *jobqueue.Job) error {
	c.jobs = append(c.jobs, job)
	return nil
}
func (c *integrationJobClient) Close() error { return nil }
```

### Test Command
```bash
go test ./notification/ -run TestIntegration -v
```

### Commit Message
```
feat(notification): Dispatcher 集成测试
```

---

## Dependency Graph

```
Task 1  (core types)
  |
  +-- Task 2  (template engine)
  |     |
  |     +-- Task 3  (dispatcher) -- depends on 1 + 2
  |           |
  |           +-- Task 4  (email sender) -- depends on 1
  |           +-- Task 5  (sms sender) -- depends on 1
  |           |     +-- Task 6  (sms providers) -- depends on 5
  |           +-- Task 7  (webhook sender) -- depends on 1
  |           +-- Task 8  (push sender) -- depends on 1
  |           |     +-- Task 9  (push providers) -- depends on 8
  |           +-- Task 10 (async) -- depends on 3
  |           +-- Task 11 (factory) -- depends on ALL senders
  |           +-- Task 12 (integration) -- depends on 3 + 10
```

Tasks 4, 5, 7, 8 can proceed in parallel after Task 3 is done.
Tasks 6 and 9 can proceed in parallel after Tasks 5 and 8 respectively.
Tasks 10, 11, 12 require earlier tasks.

---

### Critical Files for Implementation
- `/Users/tsukikage/workspace/work/servex/notification/notification.go` -- core types: Channel, Message, Result, Sender, TemplateEngine, ValidateMessage
- `/Users/tsukikage/workspace/work/servex/notification/dispatcher.go` -- Dispatcher with Register, Send, Broadcast, SendAsync (the central routing engine)
- `/Users/tsukikage/workspace/work/servex/notification/template.go` -- built-in html/template engine with dir/embed.FS loading
- `/Users/tsukikage/workspace/work/servex/notification/factory/factory.go` -- Config-driven factory that wires up all senders
- `/Users/tsukikage/workspace/work/servex/notification/email/sender.go` -- real SMTP sender using net/smtp (most complex channel implementation)