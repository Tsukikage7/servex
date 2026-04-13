package kafka

import (
	"errors"

	"github.com/IBM/sarama"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// NewPublisherFromConfig 根据 brokers 地址列表创建 Publisher，内部自动管理 sarama.Client 生命周期.
func NewPublisherFromConfig(brokers []string, log logger.Logger) (*Publisher, error) {
	if len(brokers) == 0 {
		return nil, errors.New("pubsub/kafka: brokers 不能为空")
	}

	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true

	client, err := sarama.NewClient(brokers, cfg)
	if err != nil {
		return nil, errors.Join(errors.New("pubsub/kafka: 创建 client 失败"), err)
	}

	pub, err := NewPublisher(client, WithPublisherLogger(log))
	if err != nil {
		client.Close()
		return nil, err
	}

	return pub, nil
}

// NewSubscriberFromConfig 根据 brokers 地址列表和 group 创建 Subscriber，内部自动管理 sarama.Client 生命周期.
func NewSubscriberFromConfig(brokers []string, group string, log logger.Logger) (*Subscriber, error) {
	if len(brokers) == 0 {
		return nil, errors.New("pubsub/kafka: brokers 不能为空")
	}
	if group == "" {
		return nil, errors.New("pubsub/kafka: groupID 不能为空")
	}

	cfg := sarama.NewConfig()
	cfg.Version = sarama.V0_10_2_0

	client, err := sarama.NewClient(brokers, cfg)
	if err != nil {
		return nil, errors.Join(errors.New("pubsub/kafka: 创建 client 失败"), err)
	}

	sub, err := NewSubscriber(client, group, WithSubscriberLogger(log))
	if err != nil {
		client.Close()
		return nil, err
	}

	return sub, nil
}
