// pubsub/rabbitmq/rabbitmq_test.go
package rabbitmq

import (
	"errors"
	"testing"

	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
)

func TestNewPublisher_EmptyURL(t *testing.T) {
	_, err := NewPublisher("")
	if err == nil {
		t.Fatal("expected error for empty url")
	}
}

func TestPublisher_Publish_EmptyTopic(t *testing.T) {
	// Cannot connect to real broker in unit tests; test validation logic only.
	p := &Publisher{}
	p.closed.Store(false)

	err := p.Publish(t.Context(), "", &pubsub.Message{Body: []byte("test")})
	if !errors.Is(err, pubsub.ErrEmptyTopic) {
		t.Errorf("got %v, want ErrEmptyTopic", err)
	}
}

func TestPublisher_Publish_NoMessages(t *testing.T) {
	p := &Publisher{}
	err := p.Publish(t.Context(), "test-topic")
	if !errors.Is(err, pubsub.ErrNoMessages) {
		t.Errorf("got %v, want ErrNoMessages", err)
	}
}

func TestPublisher_Publish_NilMessage(t *testing.T) {
	p := &Publisher{}
	err := p.Publish(t.Context(), "test-topic", nil)
	if !errors.Is(err, pubsub.ErrNilMessage) {
		t.Errorf("got %v, want ErrNilMessage", err)
	}
}

func TestPublisher_Publish_AfterClose(t *testing.T) {
	p := &Publisher{}
	p.closed.Store(true)

	err := p.Publish(t.Context(), "test-topic", &pubsub.Message{Body: []byte("test")})
	if !errors.Is(err, pubsub.ErrClosed) {
		t.Errorf("got %v, want ErrClosed", err)
	}
}

func TestPublisher_Close_Idempotent(t *testing.T) {
	p := &Publisher{}
	p.closed.Store(true) // Already closed
	if err := p.Close(); err != nil {
		t.Fatal("second close should not error")
	}
}

func TestNewSubscriber_EmptyURL(t *testing.T) {
	_, err := NewSubscriber("")
	if err == nil {
		t.Fatal("expected error for empty url")
	}
}

func TestSubscriber_Subscribe_EmptyTopic(t *testing.T) {
	s := &Subscriber{}
	_, err := s.Subscribe(t.Context(), "")
	if !errors.Is(err, pubsub.ErrEmptyTopic) {
		t.Errorf("got %v, want ErrEmptyTopic", err)
	}
}

func TestSubscriber_Subscribe_AfterClose(t *testing.T) {
	s := &Subscriber{}
	s.closed.Store(true)

	_, err := s.Subscribe(t.Context(), "test-topic")
	if !errors.Is(err, pubsub.ErrClosed) {
		t.Errorf("got %v, want ErrClosed", err)
	}
}

func TestSubscriber_Ack_NilMessage(t *testing.T) {
	s := &Subscriber{}
	err := s.Ack(t.Context(), nil)
	if !errors.Is(err, pubsub.ErrNilMessage) {
		t.Errorf("got %v, want ErrNilMessage", err)
	}
}

func TestSubscriber_Nack_NilMessage(t *testing.T) {
	s := &Subscriber{}
	err := s.Nack(t.Context(), nil)
	if !errors.Is(err, pubsub.ErrNilMessage) {
		t.Errorf("got %v, want ErrNilMessage", err)
	}
}

func TestSubscriber_Ack_NoDelivery(t *testing.T) {
	s := &Subscriber{}
	msg := &pubsub.Message{Metadata: map[string]any{}}
	err := s.Ack(t.Context(), msg)
	if !errors.Is(err, pubsub.ErrAckFailed) {
		t.Errorf("got %v, want ErrAckFailed", err)
	}
}

func TestSubscriber_Nack_NoDelivery(t *testing.T) {
	s := &Subscriber{}
	msg := &pubsub.Message{Metadata: map[string]any{}}
	err := s.Nack(t.Context(), msg)
	if !errors.Is(err, pubsub.ErrNackFailed) {
		t.Errorf("got %v, want ErrNackFailed", err)
	}
}

func TestSubscriber_Close_Idempotent(t *testing.T) {
	s := &Subscriber{}
	s.closed.Store(true)
	if err := s.Close(); err != nil {
		t.Fatal("second close should not error")
	}
}

func TestPublisher_ImplementsInterface(t *testing.T) {
	var _ pubsub.Publisher = (*Publisher)(nil)
}

func TestSubscriber_ImplementsInterface(t *testing.T) {
	var _ pubsub.Subscriber = (*Subscriber)(nil)
}
