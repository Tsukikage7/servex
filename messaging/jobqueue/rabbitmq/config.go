package rabbitmq

import amqp "github.com/rabbitmq/amqp091-go"

// NewStoreFromConfig 根据 AMQP URL 创建 RabbitMQ Store.
func NewStoreFromConfig(url string) (*Store, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	return NewStore(conn)
}
