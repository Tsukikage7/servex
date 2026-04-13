// notification/webhook/sender_test.go
package nwebhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tsukikage7/servex/v2/notify"
)

func TestSender_ImplementsInterface(t *testing.T) { var _ notify.Sender = (*Sender)(nil) }

func TestSender_Channel(t *testing.T) {
	s, _ := NewSender()
	if s.Channel() != notify.ChannelWebhook {
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
	msg := &notify.Message{
		Channel: notify.ChannelWebhook, To: []string{server.URL},
		Body: `{"text":"hello"}`, Metadata: map[string]string{"format": "custom"},
	}
	result, err := s.Send(t.Context(), msg)
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
	_, err := s.Send(t.Context(), &notify.Message{
		Channel: notify.ChannelWebhook, To: []string{server.URL},
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
	_, err := s.Send(t.Context(), &notify.Message{
		Channel: notify.ChannelWebhook, To: []string{server.URL},
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
	_, err := s.Send(t.Context(), &notify.Message{
		Channel: notify.ChannelWebhook, To: []string{server.URL}, Body: "test",
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
	_, err := s.Send(t.Context(), &notify.Message{
		Channel: notify.ChannelWebhook, To: []string{server.URL}, Body: "retry",
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
	_, err := s.Send(t.Context(), nil)
	if !errors.Is(err, notify.ErrNilMessage) {
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
