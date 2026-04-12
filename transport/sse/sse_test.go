package sse

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.BufferSize != 256 {
		t.Errorf("BufferSize = %d, want 256", cfg.BufferSize)
	}
	if cfg.HeartbeatInterval != 30*time.Second {
		t.Errorf("HeartbeatInterval = %v", cfg.HeartbeatInterval)
	}
	if cfg.RetryInterval != 3000 {
		t.Errorf("RetryInterval = %d, want 3000", cfg.RetryInterval)
	}
	if cfg.Headers == nil {
		t.Error("Headers should not be nil")
	}
}

func TestEventBytes(t *testing.T) {
	tests := []struct {
		name  string
		event *Event
		wants []string
	}{
		{
			"full event",
			&Event{ID: "1", Event: "msg", Data: []byte("hello"), Retry: 5000},
			[]string{"id: 1\n", "event: msg\n", "data: hello\n", "retry: 5000\n"},
		},
		{
			"data only",
			&Event{Data: []byte("payload")},
			[]string{"data: payload\n"},
		},
		{
			"empty event",
			&Event{},
			[]string{"\n"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(tt.event.Bytes())
			for _, want := range tt.wants {
				if !strings.Contains(got, want) {
					t.Errorf("Bytes() = %q, missing %q", got, want)
				}
			}
			// Every event ends with double newline.
			if !strings.HasSuffix(got, "\n") {
				t.Error("event should end with newline")
			}
		})
	}
}

func TestNewServerNilConfig(t *testing.T) {
	s := NewServer(nil)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.Count() != 0 {
		t.Errorf("Count = %d, want 0", s.Count())
	}
}

func TestNewServerWithConfig(t *testing.T) {
	cfg := &Config{
		BufferSize:        64,
		HeartbeatInterval: 10 * time.Second,
		RetryInterval:     1000,
		Headers:           map[string]string{"X-Custom": "value"},
	}
	s := NewServer(cfg)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
}

func TestServerRunAndClose(t *testing.T) {
	s := NewServer(nil)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- s.Run(ctx)
	}()

	// Let the run loop start.
	time.Sleep(10 * time.Millisecond)

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run error: %v", err)
	}
}

func TestServerSendToUnknownClient(t *testing.T) {
	s := NewServer(nil)
	err := s.Send("nonexistent", &Event{Data: []byte("test")})
	if err != ErrClientNotFound {
		t.Fatalf("expected ErrClientNotFound, got %v", err)
	}
}

func TestServerCallbacks(t *testing.T) {
	s := NewServer(nil)
	connected := false
	disconnected := false

	s.OnConnect(func(c Client) {
		connected = true
	})
	s.OnDisconnect(func(c Client) {
		disconnected = true
	})

	// Verify callbacks are set without panic.
	if connected || disconnected {
		t.Fatal("callbacks should not have fired yet")
	}
}

func TestClientMetadata(t *testing.T) {
	c := newClient(16, t.Context())
	defer c.Close()

	c.SetMetadata("key", "value")
	meta := c.Metadata()
	if meta["key"] != "value" {
		t.Errorf("Metadata[key] = %v, want value", meta["key"])
	}

	if c.ID() == "" {
		t.Error("client ID should not be empty")
	}
}

func TestClientSendAndClose(t *testing.T) {
	c := newClient(2, t.Context())

	err := c.Send(&Event{Data: []byte("msg1")})
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}

	_ = c.Close()
	// Close again should be idempotent.
	_ = c.Close()

	err = c.Send(&Event{Data: []byte("after close")})
	if err != ErrConnectionClosed {
		t.Fatalf("expected ErrConnectionClosed, got %v", err)
	}
}

// --- Event construction helpers ---

func TestNewTextEvent(t *testing.T) {
	e := NewTextEvent("notify", "hello world")
	if e.Event != "notify" {
		t.Errorf("Event = %q, want notify", e.Event)
	}
	if string(e.Data) != "hello world" {
		t.Errorf("Data = %q, want 'hello world'", e.Data)
	}
}

func TestNewJSONEvent(t *testing.T) {
	e, err := NewJSONEvent("update", map[string]int{"count": 42})
	if err != nil {
		t.Fatalf("NewJSONEvent error: %v", err)
	}
	if e.Event != "update" {
		t.Errorf("Event = %q, want update", e.Event)
	}
	if !strings.Contains(string(e.Data), "42") {
		t.Errorf("Data should contain '42', got %q", e.Data)
	}
}

func TestNewJSONEvent_InvalidData(t *testing.T) {
	// Channels cannot be marshaled to JSON.
	_, err := NewJSONEvent("bad", make(chan int))
	if err == nil {
		t.Error("expected error for non-marshalable data")
	}
}

func TestMustNewJSONEvent_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for non-marshalable data")
		}
	}()
	MustNewJSONEvent("bad", make(chan int))
}

func TestMustNewJSONEvent_Success(t *testing.T) {
	e := MustNewJSONEvent("ok", map[string]string{"key": "value"})
	if e.Event != "ok" {
		t.Errorf("Event = %q, want ok", e.Event)
	}
}

func TestNewEvent(t *testing.T) {
	e := NewEvent("test", []byte("data"))
	if e.Event != "test" || string(e.Data) != "data" {
		t.Errorf("NewEvent = %+v", e)
	}
}

func TestNewEventWithID(t *testing.T) {
	e := NewEventWithID("123", "test", []byte("data"))
	if e.ID != "123" || e.Event != "test" {
		t.Errorf("NewEventWithID = %+v", e)
	}
}

func TestNewMessageEvent(t *testing.T) {
	e := NewMessageEvent("hi")
	if e.Event != "message" || string(e.Data) != "hi" {
		t.Errorf("NewMessageEvent = %+v", e)
	}
}

// --- EventBuilder ---

func TestEventBuilder(t *testing.T) {
	e := NewBuilder().
		ID("evt-1").
		Event("update").
		Text("hello").
		Retry(5000).
		Build()

	if e.ID != "evt-1" {
		t.Errorf("ID = %q", e.ID)
	}
	if e.Event != "update" {
		t.Errorf("Event = %q", e.Event)
	}
	if string(e.Data) != "hello" {
		t.Errorf("Data = %q", e.Data)
	}
	if e.Retry != 5000 {
		t.Errorf("Retry = %d", e.Retry)
	}
}

func TestEventBuilder_JSON(t *testing.T) {
	e := NewBuilder().
		Event("json-event").
		JSON(map[string]int{"x": 1}).
		Build()

	if !strings.Contains(string(e.Data), `"x":1`) {
		t.Errorf("JSON data = %q", e.Data)
	}
}

func TestEventBuilder_JSON_Error(t *testing.T) {
	e := NewBuilder().
		JSON(make(chan int)).
		Build()

	if !strings.Contains(string(e.Data), "error") {
		t.Errorf("expected error in JSON data, got %q", e.Data)
	}
}

func TestEventBuilder_Data(t *testing.T) {
	e := NewBuilder().Data([]byte("raw")).Build()
	if string(e.Data) != "raw" {
		t.Errorf("Data = %q", e.Data)
	}
}

// --- Event Bytes serialization edge cases ---

func TestEventBytes_RetryOnly(t *testing.T) {
	e := &Event{Retry: 1000}
	got := string(e.Bytes())
	if !strings.Contains(got, "retry: 1000") {
		t.Errorf("expected retry field, got %q", got)
	}
}

func TestEventBytes_IDOnly(t *testing.T) {
	e := &Event{ID: "42"}
	got := string(e.Bytes())
	if !strings.Contains(got, "id: 42") {
		t.Errorf("expected id field, got %q", got)
	}
}

// --- Broker ---

func TestBroker_SubscribeAndPublish(t *testing.T) {
	s := NewServer(nil)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	time.Sleep(20 * time.Millisecond)

	// We cannot easily get a client registered without HTTP,
	// so test Broker with a non-existent client for Subscribe error.
	broker := NewBroker(s)

	err := broker.Subscribe("nonexistent", "topic1")
	if err != ErrClientNotFound {
		t.Fatalf("expected ErrClientNotFound, got %v", err)
	}

	if broker.Subscribers("topic1") != 0 {
		t.Error("expected 0 subscribers")
	}

	cancel()
	<-done
}

func TestBroker_UnsubscribeAll(t *testing.T) {
	s := NewServer(nil)
	broker := NewBroker(s)
	// Just ensure it does not panic on empty state.
	broker.UnsubscribeAll("nonexistent")
	broker.Unsubscribe("nonexistent", "topic")
}

// --- Server direct operations (without HTTP) ---

func TestServerCloseIdempotent(t *testing.T) {
	s := NewServer(nil)
	if err := s.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close error: %v", err)
	}
}

func TestServerBroadcastNoClients(t *testing.T) {
	s := NewServer(nil)
	// Should not panic.
	s.Broadcast(&Event{Data: []byte("test")})
}

func TestServerBroadcastToNoClients(t *testing.T) {
	s := NewServer(nil)
	// Should not panic.
	s.BroadcastTo([]string{"nonexistent"}, &Event{Data: []byte("test")})
}

func TestServerClientLookup(t *testing.T) {
	s := NewServer(nil)
	_, ok := s.Client("no-such")
	if ok {
		t.Error("expected false for nonexistent client")
	}
}

func TestServerClients_Empty(t *testing.T) {
	s := NewServer(nil)
	if clients := s.Clients(); len(clients) != 0 {
		t.Errorf("Clients len = %d, want 0", len(clients))
	}
}

func TestServerErrors(t *testing.T) {
	errs := []error{
		ErrClientNotFound,
		ErrServerClosed,
		ErrConnectionClosed,
		ErrNotFlusher,
	}
	for _, e := range errs {
		if e == nil || e.Error() == "" {
			t.Errorf("error should not be nil/empty: %v", e)
		}
	}
}

// --- Client context ---

func TestClientContext(t *testing.T) {
	c := newClient(4, t.Context())
	defer c.Close()
	ctx := c.Context()
	if ctx == nil {
		t.Error("client context should not be nil")
	}
}

func TestClientSendBufferFull(t *testing.T) {
	c := newClient(1, t.Context()) // buffer size 1
	defer c.Close()

	// Fill buffer.
	_ = c.Send(&Event{Data: []byte("first")})
	// Second send to full buffer should not error (drops silently).
	err := c.Send(&Event{Data: []byte("second")})
	if err != nil {
		t.Errorf("expected nil error on full buffer, got %v", err)
	}
}
