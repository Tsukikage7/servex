// notification/sms/sender_test.go
package sms

import (
	"context"
	"errors"
	"testing"

	"github.com/Tsukikage7/servex/v2/notify"
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
	var _ notify.Sender = (*Sender)(nil)
}

func TestSender_Channel(t *testing.T) {
	s, _ := NewSender(&mockProvider{name: "mock"})
	if s.Channel() != notify.ChannelSMS {
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
	result, err := s.Send(t.Context(), &notify.Message{
		Channel: notify.ChannelSMS, To: []string{"13800138000"}, Body: "验证码：1234",
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
	_, err := s.Send(t.Context(), &notify.Message{
		Channel: notify.ChannelSMS, To: []string{"13800138000"},
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
	_, err := s.Send(t.Context(), &notify.Message{
		Channel: notify.ChannelSMS, To: []string{"1", "2", "3"}, Body: "test",
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
	_, err := s.Send(t.Context(), &notify.Message{Channel: notify.ChannelSMS, To: []string{"1"}, Body: "x"})
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
