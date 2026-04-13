// pubsub/kafka/subscriber_test.go
package kafka

import (
	"errors"
	"testing"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"

	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
)

func TestNewSubscriber_NilClient(t *testing.T) {
	_, err := NewSubscriber(nil, "group")
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestNewSubscriber_EmptyGroup(t *testing.T) {
	cfg := newTestConfig()
	client := newMockClient(t, cfg)
	defer client.Close()

	_, err := NewSubscriber(client, "")
	if err == nil {
		t.Fatal("expected error for empty group")
	}
}

func TestSubscriber_Subscribe_EmptyTopic(t *testing.T) {
	cfg := newTestConfig()
	client := newMockClient(t, cfg)
	defer client.Close()

	sub, err := NewSubscriber(client, "test-group")
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	_, err = sub.Subscribe(t.Context(), "")
	if !errors.Is(err, pubsub.ErrEmptyTopic) {
		t.Errorf("got %v, want ErrEmptyTopic", err)
	}
}

func TestSubscriber_Subscribe_AfterClose(t *testing.T) {
	cfg := newTestConfig()
	client := newMockClient(t, cfg)
	defer client.Close()

	sub, err := NewSubscriber(client, "test-group")
	if err != nil {
		t.Fatal(err)
	}
	sub.Close()

	_, err = sub.Subscribe(t.Context(), "test-topic")
	if !errors.Is(err, pubsub.ErrClosed) {
		t.Errorf("got %v, want ErrClosed", err)
	}
}

func TestSubscriber_Ack_NilMessage(t *testing.T) {
	cfg := newTestConfig()
	client := newMockClient(t, cfg)
	defer client.Close()

	sub, err := NewSubscriber(client, "test-group")
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	err = sub.Ack(t.Context(), nil)
	if !errors.Is(err, pubsub.ErrNilMessage) {
		t.Errorf("got %v, want ErrNilMessage", err)
	}
}

func TestSubscriber_Close_Idempotent(t *testing.T) {
	cfg := newTestConfig()
	client := newMockClient(t, cfg)
	defer client.Close()

	sub, err := NewSubscriber(client, "test-group")
	if err != nil {
		t.Fatal(err)
	}

	if err := sub.Close(); err != nil {
		t.Fatal(err)
	}
	if err := sub.Close(); err != nil {
		t.Fatal("second close should not error")
	}
}

func TestSubscriber_ImplementsInterface(t *testing.T) {
	var _ pubsub.Subscriber = (*Subscriber)(nil)
}

func newTestConfig() *sarama.Config {
	cfg := mocks.NewTestConfig()
	cfg.Producer.Return.Successes = true
	cfg.Version = sarama.V0_10_2_0
	return cfg
}
