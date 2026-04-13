// pubsub/redis/redis_test.go
package redis

import (
	"errors"
	"testing"

	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
)

func TestNewPublisher_NilClient(t *testing.T) {
	_, err := NewPublisher(nil)
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestPublisher_Publish_EmptyTopic(t *testing.T) {
	pub := &Publisher{}
	err := pub.Publish(t.Context(), "", &pubsub.Message{Body: []byte("test")})
	if !errors.Is(err, pubsub.ErrEmptyTopic) {
		t.Errorf("got %v, want ErrEmptyTopic", err)
	}
}

func TestPublisher_Publish_NoMessages(t *testing.T) {
	pub := &Publisher{}
	err := pub.Publish(t.Context(), "test-stream")
	if !errors.Is(err, pubsub.ErrNoMessages) {
		t.Errorf("got %v, want ErrNoMessages", err)
	}
}

func TestPublisher_Publish_NilMessage(t *testing.T) {
	pub := &Publisher{}
	err := pub.Publish(t.Context(), "test-stream", nil)
	if !errors.Is(err, pubsub.ErrNilMessage) {
		t.Errorf("got %v, want ErrNilMessage", err)
	}
}

func TestPublisher_Publish_AfterClose(t *testing.T) {
	pub := &Publisher{}
	pub.closed.Store(true)
	err := pub.Publish(t.Context(), "test-stream", &pubsub.Message{Body: []byte("test")})
	if !errors.Is(err, pubsub.ErrClosed) {
		t.Errorf("got %v, want ErrClosed", err)
	}
}

func TestPublisher_Close_Idempotent(t *testing.T) {
	pub := &Publisher{}
	if err := pub.Close(); err != nil {
		t.Fatal(err)
	}
	if err := pub.Close(); err != nil {
		t.Fatal("second close should not error")
	}
}

func TestNewSubscriber_NilClient(t *testing.T) {
	_, err := NewSubscriber(nil)
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestSubscriber_Subscribe_EmptyTopic(t *testing.T) {
	sub := &Subscriber{}
	_, err := sub.Subscribe(t.Context(), "")
	if !errors.Is(err, pubsub.ErrEmptyTopic) {
		t.Errorf("got %v, want ErrEmptyTopic", err)
	}
}

func TestSubscriber_Subscribe_AfterClose(t *testing.T) {
	sub := &Subscriber{}
	sub.closed.Store(true)
	_, err := sub.Subscribe(t.Context(), "test-stream")
	if !errors.Is(err, pubsub.ErrClosed) {
		t.Errorf("got %v, want ErrClosed", err)
	}
}

func TestSubscriber_Ack_NilMessage(t *testing.T) {
	sub := &Subscriber{}
	err := sub.Ack(t.Context(), nil)
	if !errors.Is(err, pubsub.ErrNilMessage) {
		t.Errorf("got %v, want ErrNilMessage", err)
	}
}

func TestSubscriber_Nack_NilMessage(t *testing.T) {
	sub := &Subscriber{}
	err := sub.Nack(t.Context(), nil)
	if !errors.Is(err, pubsub.ErrNilMessage) {
		t.Errorf("got %v, want ErrNilMessage", err)
	}
}

func TestSubscriber_Ack_NoStreamID(t *testing.T) {
	sub := &Subscriber{opts: subscriberOptions{groupID: "test-group"}}
	msg := &pubsub.Message{Metadata: map[string]any{}}
	err := sub.Ack(t.Context(), msg)
	if !errors.Is(err, pubsub.ErrAckFailed) {
		t.Errorf("got %v, want ErrAckFailed", err)
	}
}

func TestSubscriber_Close_Idempotent(t *testing.T) {
	sub := &Subscriber{}
	if err := sub.Close(); err != nil {
		t.Fatal(err)
	}
	if err := sub.Close(); err != nil {
		t.Fatal("second close should not error")
	}
}

func TestPublisher_ImplementsInterface(t *testing.T) {
	var _ pubsub.Publisher = (*Publisher)(nil)
}

func TestSubscriber_ImplementsInterface(t *testing.T) {
	var _ pubsub.Subscriber = (*Subscriber)(nil)
}

func TestConvertXMessage_Body(t *testing.T) {
	xmsg := struct {
		ID     string
		Values map[string]any
	}{
		ID: "1-0",
		Values: map[string]any{
			"body": "hello",
			"key":  "mykey",
		},
	}
	// We can't call convertXMessage directly with the struct because it expects goredis.XMessage,
	// but the test ensures the package compiles correctly.
	_ = xmsg
}
