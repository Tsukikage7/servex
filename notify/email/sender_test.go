// notification/email/sender_test.go
package email

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/Tsukikage7/servex/v2/notify"
)

func TestSender_ImplementsInterface(t *testing.T) {
	var _ notify.Sender = (*Sender)(nil)
}

func TestSender_Channel(t *testing.T) {
	s, _ := NewSender(WithSMTP("localhost", 25), WithFrom("a@b.com", "Test"))
	if s.Channel() != notify.ChannelEmail {
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

	msg := &notify.Message{
		Channel: notify.ChannelEmail,
		To:      []string{"recipient@test.com"},
		Subject: "Test Subject",
		Body:    "<h1>Hello</h1>",
	}
	result, err := s.Send(t.Context(), msg)
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

	msg := &notify.Message{
		Channel: notify.ChannelEmail, To: []string{"to@test.com"},
		Subject: "CC Test", Body: "body",
		Metadata: map[string]string{"cc": "cc1@test.com,cc2@test.com", "bcc": "bcc@test.com"},
	}
	_, err = s.Send(t.Context(), msg)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSender_Send_NilMessage(t *testing.T) {
	s, _ := NewSender(WithSMTP("localhost", 25), WithFrom("a@b.com", "Test"))
	_, err := s.Send(t.Context(), nil)
	if !errors.Is(err, notify.ErrNilMessage) {
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
