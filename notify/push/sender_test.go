// notification/push/sender_test.go
package push

import (
	"context"
	"errors"
	"testing"

	"github.com/Tsukikage7/servex/v2/notify"
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

func TestSender_ImplementsInterface(t *testing.T) { var _ notify.Sender = (*Sender)(nil) }

func TestSender_Channel(t *testing.T) {
	s, _ := NewSender(&mockProvider{name: "mock"})
	if s.Channel() != notify.ChannelPush {
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
	result, err := s.Send(t.Context(), &notify.Message{
		Channel: notify.ChannelPush, To: []string{"device-token"},
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
	_, err := s.Send(t.Context(), &notify.Message{
		Channel: notify.ChannelPush, To: []string{"t1", "t2"}, Body: "test",
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
	_, err := s.Send(t.Context(), &notify.Message{
		Channel: notify.ChannelPush, To: []string{"t"}, Body: "test",
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestSender_Send_NilMessage(t *testing.T) {
	s, _ := NewSender(&mockProvider{name: "mock"})
	_, err := s.Send(t.Context(), nil)
	if !errors.Is(err, notify.ErrNilMessage) {
		t.Errorf("got %v, want ErrNilMessage", err)
	}
}
