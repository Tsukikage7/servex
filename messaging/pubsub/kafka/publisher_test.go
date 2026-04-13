// pubsub/kafka/publisher_test.go
package kafka

import (
	"errors"
	"testing"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"

	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
)

func TestNewPublisher_NilClient(t *testing.T) {
	_, err := NewPublisher(nil)
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestPublisher_Publish_EmptyTopic(t *testing.T) {
	cfg := mocks.NewTestConfig()
	cfg.Producer.Return.Successes = true
	client := newMockClient(t, cfg)
	defer client.Close()

	pub, err := NewPublisher(client)
	if err != nil {
		t.Fatal(err)
	}
	defer pub.Close()

	err = pub.Publish(t.Context(), "", &pubsub.Message{Body: []byte("test")})
	if !errors.Is(err, pubsub.ErrEmptyTopic) {
		t.Errorf("got %v, want ErrEmptyTopic", err)
	}
}

func TestPublisher_Publish_NoMessages(t *testing.T) {
	cfg := mocks.NewTestConfig()
	cfg.Producer.Return.Successes = true
	client := newMockClient(t, cfg)
	defer client.Close()

	pub, err := NewPublisher(client)
	if err != nil {
		t.Fatal(err)
	}
	defer pub.Close()

	err = pub.Publish(t.Context(), "test-topic")
	if !errors.Is(err, pubsub.ErrNoMessages) {
		t.Errorf("got %v, want ErrNoMessages", err)
	}
}

func TestPublisher_Publish_NilMessage(t *testing.T) {
	cfg := mocks.NewTestConfig()
	cfg.Producer.Return.Successes = true
	client := newMockClient(t, cfg)
	defer client.Close()

	pub, err := NewPublisher(client)
	if err != nil {
		t.Fatal(err)
	}
	defer pub.Close()

	err = pub.Publish(t.Context(), "test-topic", nil)
	if !errors.Is(err, pubsub.ErrNilMessage) {
		t.Errorf("got %v, want ErrNilMessage", err)
	}
}

func TestPublisher_Publish_AfterClose(t *testing.T) {
	cfg := mocks.NewTestConfig()
	cfg.Producer.Return.Successes = true
	client := newMockClient(t, cfg)
	defer client.Close()

	pub, err := NewPublisher(client)
	if err != nil {
		t.Fatal(err)
	}
	pub.Close()

	err = pub.Publish(t.Context(), "test-topic", &pubsub.Message{Body: []byte("test")})
	if !errors.Is(err, pubsub.ErrClosed) {
		t.Errorf("got %v, want ErrClosed", err)
	}
}

func TestPublisher_Close_Idempotent(t *testing.T) {
	cfg := mocks.NewTestConfig()
	cfg.Producer.Return.Successes = true
	client := newMockClient(t, cfg)
	defer client.Close()

	pub, err := NewPublisher(client)
	if err != nil {
		t.Fatal(err)
	}

	if err := pub.Close(); err != nil {
		t.Fatal(err)
	}
	if err := pub.Close(); err != nil {
		t.Fatal("second close should not error")
	}
}

func TestPublisher_ImplementsInterface(t *testing.T) {
	var _ pubsub.Publisher = (*Publisher)(nil)
}

func newMockClient(t *testing.T, cfg *sarama.Config) sarama.Client {
	t.Helper()
	broker := sarama.NewMockBroker(t, 0)
	broker.SetHandlerByMap(map[string]sarama.MockResponse{
		"ApiVersionsRequest": sarama.NewMockApiVersionsResponse(t),
		"MetadataRequest": sarama.NewMockMetadataResponse(t).
			SetBroker(broker.Addr(), broker.BrokerID()).
			SetLeader("test-topic", 0, broker.BrokerID()),
		"ProduceRequest": sarama.NewMockProduceResponse(t),
	})
	cfg.Metadata.Retry.Max = 0
	client, err := sarama.NewClient([]string{broker.Addr()}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { broker.Close() })
	return client
}
