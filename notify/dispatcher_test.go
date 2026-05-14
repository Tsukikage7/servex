package notify

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

	result, err := d.Send(t.Context(), &Message{Channel: ChannelEmail, To: []string{"a@b.com"}, Body: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if result.MessageID != "mock-id" {
		t.Errorf("messageID = %q", result.MessageID)
	}
}

func TestDispatcher_Send_NoSender(t *testing.T) {
	d := NewDispatcher()
	_, err := d.Send(t.Context(), &Message{Channel: ChannelEmail, To: []string{"a@b.com"}, Body: "hi"})
	if !errors.Is(err, ErrNoSender) {
		t.Errorf("got %v, want ErrNoSender", err)
	}
}

func TestDispatcher_Send_InvalidMessage(t *testing.T) {
	d := NewDispatcher()
	_, err := d.Send(t.Context(), nil)
	if !errors.Is(err, ErrNilMessage) {
		t.Errorf("got %v, want ErrNilMessage", err)
	}
}

func TestDispatcher_Send_WithDefaultChannel(t *testing.T) {
	d := NewDispatcher(WithDefaultChannel(ChannelSMS))
	d.Register(newMockSender(ChannelSMS))

	result, err := d.Send(t.Context(), &Message{To: []string{"13800138000"}, Body: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Channel != ChannelSMS {
		t.Errorf("channel = %q, want sms", result.Channel)
	}
}

func TestDispatcher_Send_WithTemplateEngine(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "welcome.html"), []byte("Hello, {{.Name}}!"), 0o644)

	eng := NewTemplateEngine(WithTemplateDir(dir))
	d := NewDispatcher(WithTemplateEngine(eng))
	s := newMockSender(ChannelEmail)
	var capturedBody string
	s.sendFunc = func(_ context.Context, msg *Message) (*Result, error) {
		capturedBody = msg.Body
		return &Result{MessageID: "t-1", Channel: ChannelEmail}, nil
	}
	d.Register(s)

	_, err := d.Send(t.Context(), &Message{
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

	results := d.Broadcast(t.Context(), []Channel{ChannelEmail, ChannelSMS}, &Message{To: []string{"user"}, Body: "alert"})
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

	results := d.Broadcast(t.Context(), []Channel{ChannelEmail, ChannelSMS}, &Message{To: []string{"user"}, Body: "alert"})
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
	_, err := d.Send(t.Context(), &Message{Channel: ChannelEmail, To: []string{"a@b.com"}, Body: "hi"})
	if !errors.Is(err, ErrClosed) {
		t.Errorf("got %v, want ErrClosed", err)
	}
}
