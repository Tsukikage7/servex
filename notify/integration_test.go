// notification/integration_test.go
package notify

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
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
	ctx := t.Context()

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

	d.Send(t.Context(), &Message{
		Channel: ChannelEmail, To: []string{"u@x.com"},
		TemplateID: "order.html", TemplateData: map[string]any{"ID": "12345"},
	})
	if msg := rec.last(); msg.Body != "Order #12345 confirmed." {
		t.Errorf("body = %q", msg.Body)
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
