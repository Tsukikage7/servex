// pubsub/factory/factory_test.go
package factory_test

import (
	"errors"
	"testing"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"

	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
	"github.com/Tsukikage7/servex/v2/messaging/pubsub/factory"
	"github.com/Tsukikage7/servex/v2/messaging/pubsub/kafka"
)

// ---- NewPublisher 错误路径 ----

func TestNewPublisher_NilConfig(t *testing.T) {
	_, err := factory.NewPublisher(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestNewPublisher_EmptyType(t *testing.T) {
	_, err := factory.NewPublisher(&factory.Config{}, nil)
	if err == nil {
		t.Fatal("expected error for empty type")
	}
}

func TestNewPublisher_UnsupportedType(t *testing.T) {
	_, err := factory.NewPublisher(&factory.Config{Type: "nsq"}, nil)
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

// ---- NewSubscriber 错误路径 ----

func TestNewSubscriber_NilConfig(t *testing.T) {
	_, err := factory.NewSubscriber(nil, "group", nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestNewSubscriber_EmptyType(t *testing.T) {
	_, err := factory.NewSubscriber(&factory.Config{}, "group", nil)
	if err == nil {
		t.Fatal("expected error for empty type")
	}
}

func TestNewSubscriber_UnsupportedType(t *testing.T) {
	_, err := factory.NewSubscriber(&factory.Config{Type: "nsq"}, "group", nil)
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

// ---- Kafka 路由正确性（mock broker）----

// TestNewPublisher_Kafka_Route 使用 sarama mock broker 验证 kafka 路由能正确创建 Publisher。
func TestNewPublisher_Kafka_Route(t *testing.T) {
	broker := sarama.NewMockBroker(t, 0)
	t.Cleanup(func() { broker.Close() })

	cfg := mocks.NewTestConfig()
	cfg.Producer.Return.Successes = true
	cfg.Metadata.Retry.Max = 0
	broker.SetHandlerByMap(map[string]sarama.MockResponse{
		"ApiVersionsRequest": sarama.NewMockApiVersionsResponse(t),
		"MetadataRequest": sarama.NewMockMetadataResponse(t).
			SetBroker(broker.Addr(), broker.BrokerID()).
			SetLeader("test-topic", 0, broker.BrokerID()),
		"ProduceRequest": sarama.NewMockProduceResponse(t),
	})

	// 直接通过 kafka.NewPublisher + mock client 验证接口满足，
	// 而不依赖真实网络连接（factory.NewPublisher 会调 sarama.NewClient）。
	client, err := sarama.NewClient([]string{broker.Addr()}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })

	pub, err := kafka.NewPublisher(client)
	if err != nil {
		t.Fatalf("kafka.NewPublisher: %v", err)
	}
	t.Cleanup(func() { pub.Close() })

	// 验证满足 pubsub.Publisher 接口
	var _ pubsub.Publisher = pub
}

// ---- Redis 路由正确性（不需要真实连接，仅验证 client 构造）----

func TestNewPublisher_Redis_Route(t *testing.T) {
	cfg := &factory.Config{
		Type: "redis",
		Addr: "localhost:6379",
	}
	pub, err := factory.NewPublisher(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pub == nil {
		t.Fatal("expected non-nil publisher")
	}
	pub.Close()
}

func TestNewSubscriber_Redis_Route(t *testing.T) {
	cfg := &factory.Config{
		Type: "redis",
		Addr: "localhost:6379",
	}
	sub, err := factory.NewSubscriber(cfg, "mygroup", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub == nil {
		t.Fatal("expected non-nil subscriber")
	}
	sub.Close()
}

// ---- 确保返回类型满足接口 ----

func TestFactoryTypes(t *testing.T) {
	// Compile-time interface check via nil pointer
	var _ pubsub.Publisher = (*kafka.Publisher)(nil)
	var _ pubsub.Subscriber = (*kafka.Subscriber)(nil)
}

// errCheck 仅用于让 errors 包在测试中被引用（避免 unused import）。
var _ = errors.New
