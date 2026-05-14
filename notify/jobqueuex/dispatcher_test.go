package jobqueuex

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Tsukikage7/servex/v2/messaging/jobqueue"
	"github.com/Tsukikage7/servex/v2/notify"
)

type mockJobClient struct {
	jobs   []*jobqueue.Job
	closed bool
}

func (m *mockJobClient) Enqueue(_ context.Context, job *jobqueue.Job) error {
	m.jobs = append(m.jobs, job)
	return nil
}

func (m *mockJobClient) Close() error {
	m.closed = true
	return nil
}

func TestDispatcher_SendAsync(t *testing.T) {
	client := &mockJobClient{}
	d := NewDispatcher(client)

	msg := &notify.Message{Channel: notify.ChannelEmail, To: []string{"a@b.com"}, Subject: "Async", Body: "hello"}
	if err := d.SendAsync(t.Context(), msg); err != nil {
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

func TestDispatcher_SendAsync_DefaultChannel(t *testing.T) {
	client := &mockJobClient{}
	d := NewDispatcher(client, WithDefaultChannel(notify.ChannelSMS))

	msg := &notify.Message{To: []string{"13800138000"}, Body: "hi"}
	if err := d.SendAsync(t.Context(), msg); err != nil {
		t.Fatal(err)
	}

	var decoded notify.Message
	if err := json.Unmarshal(client.jobs[0].Payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Channel != notify.ChannelSMS {
		t.Fatalf("channel = %q, want sms", decoded.Channel)
	}
	if msg.Channel != "" {
		t.Fatalf("original message was mutated: %+v", msg)
	}
}

func TestDispatcher_SendAsync_NoJobQueue(t *testing.T) {
	d := NewDispatcher(nil)
	err := d.SendAsync(t.Context(), &notify.Message{Channel: notify.ChannelEmail, To: []string{"a@b.com"}, Body: "hi"})
	if !errors.Is(err, notify.ErrJobQueueNotConfigured) {
		t.Errorf("got %v, want ErrJobQueueNotConfigured", err)
	}
}

func TestDispatcher_SendAsync_InvalidMessage(t *testing.T) {
	client := &mockJobClient{}
	d := NewDispatcher(client)
	err := d.SendAsync(t.Context(), nil)
	if !errors.Is(err, notify.ErrNilMessage) {
		t.Errorf("got %v, want ErrNilMessage", err)
	}
}

func TestDispatcher_Close(t *testing.T) {
	client := &mockJobClient{}
	d := NewDispatcher(client)
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if !client.closed {
		t.Fatal("client should be closed")
	}
	err := d.SendAsync(t.Context(), &notify.Message{Channel: notify.ChannelEmail, To: []string{"a@b.com"}, Body: "hi"})
	if !errors.Is(err, notify.ErrClosed) {
		t.Errorf("got %v, want ErrClosed", err)
	}
}
