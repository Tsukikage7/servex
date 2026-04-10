package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/Tsukikage7/servex/messaging/pubsub"
)

// Subscriber 通过 RabbitMQ 订阅消息.
type Subscriber struct {
	url     string
	conn    *amqp.Connection
	ch      *amqp.Channel
	mu      sync.Mutex
	closed  atomic.Bool
	wg      sync.WaitGroup
	cancels []context.CancelFunc // 所有订阅的 cancel 函数
	opts    subscriberOptions
}

// NewSubscriber 基于 AMQP URL 创建 Subscriber.
func NewSubscriber(url string, opts ...SubscriberOption) (*Subscriber, error) {
	if url == "" {
		return nil, errors.New("pubsub/rabbitmq: url 不能为空")
	}

	o := subscriberOptions{
		exchangeType:  "direct",
		durable:       true,
		autoAck:       false,
		prefetchCount: 10,
	}
	for _, opt := range opts {
		opt(&o)
	}

	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("pubsub/rabbitmq: 连接失败: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("pubsub/rabbitmq: 创建 channel 失败: %w", err)
	}

	if o.prefetchCount > 0 {
		if err := ch.Qos(o.prefetchCount, 0, false); err != nil {
			ch.Close()
			conn.Close()
			return nil, fmt.Errorf("pubsub/rabbitmq: 设置 QoS 失败: %w", err)
		}
	}

	if o.exchange != "" {
		if err := ch.ExchangeDeclare(
			o.exchange,
			o.exchangeType,
			o.durable,
			false, false, false, nil,
		); err != nil {
			ch.Close()
			conn.Close()
			return nil, fmt.Errorf("pubsub/rabbitmq: 声明交换机失败: %w", err)
		}
	}

	s := &Subscriber{url: url, conn: conn, ch: ch, opts: o}

	// 启动连接断开监听和自动重连
	go s.reconnectLoop()

	return s, nil
}

// reconnectLoop 监听连接关闭通知，自动重连（指数退避）.
func (s *Subscriber) reconnectLoop() {
	for {
		s.mu.Lock()
		conn := s.conn
		s.mu.Unlock()

		if conn == nil || s.closed.Load() {
			return
		}

		notifyClose := conn.NotifyClose(make(chan *amqp.Error, 1))
		amqpErr, ok := <-notifyClose
		if !ok || s.closed.Load() {
			return
		}

		s.logf("连接断开: %v，开始重连", amqpErr)

		backoff := time.Second
		const maxBackoff = 30 * time.Second
		for !s.closed.Load() {
			newConn, err := amqp.Dial(s.url)
			if err != nil {
				s.logf("重连失败: %v，%v 后重试", err, backoff)
				time.Sleep(backoff)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}

			newCh, err := newConn.Channel()
			if err != nil {
				s.logf("重连后创建 channel 失败: %v", err)
				newConn.Close()
				continue
			}

			if s.opts.prefetchCount > 0 {
				if err := newCh.Qos(s.opts.prefetchCount, 0, false); err != nil {
					s.logf("重连后设置 QoS 失败: %v", err)
					newCh.Close()
					newConn.Close()
					continue
				}
			}

			s.mu.Lock()
			s.conn = newConn
			s.ch = newCh
			s.mu.Unlock()

			s.logf("重连成功")
			break
		}
	}
}

func (s *Subscriber) logf(format string, args ...any) {
	if s.opts.logger != nil {
		s.opts.logger.Infof("[RabbitMQ/Subscriber] "+format, args...)
	}
}

// Subscribe 订阅指定 queue/topic，返回消息 channel.
// topic 作为 queue 名称（若有 exchange 则作为 routing key）.
func (s *Subscriber) Subscribe(ctx context.Context, topic string) (<-chan *pubsub.Message, error) {
	if s.closed.Load() {
		return nil, pubsub.ErrClosed
	}
	if topic == "" {
		return nil, pubsub.ErrEmptyTopic
	}

	s.mu.Lock()
	ch := s.ch
	s.mu.Unlock()

	// 声明队列
	queue, err := ch.QueueDeclare(topic, s.opts.durable, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("pubsub/rabbitmq: 声明队列失败: %w", err)
	}

	// 若有交换机，绑定队列
	if s.opts.exchange != "" {
		if err := ch.QueueBind(queue.Name, topic, s.opts.exchange, false, nil); err != nil {
			return nil, fmt.Errorf("pubsub/rabbitmq: 绑定队列失败: %w", err)
		}
	}

	deliveries, err := ch.Consume(queue.Name, "", s.opts.autoAck, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("pubsub/rabbitmq: 启动消费失败: %w", err)
	}

	msgCh := make(chan *pubsub.Message)
	subCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancels = append(s.cancels, cancel)
	s.mu.Unlock()

	s.wg.Go(func() {
		defer close(msgCh)
		for {
			select {
			case <-subCtx.Done():
				return
			case d, ok := <-deliveries:
				if !ok {
					return
				}
				headers := make(map[string]string, len(d.Headers))
				for k, v := range d.Headers {
					if str, ok := v.(string); ok {
						headers[k] = str
					}
				}
				msg := &pubsub.Message{
					Topic:   d.RoutingKey,
					Key:     []byte(d.MessageId),
					Body:    d.Body,
					Headers: headers,
					Metadata: map[string]any{
						"delivery_tag": d.DeliveryTag,
						"delivery":     d,
					},
				}
				select {
				case msgCh <- msg:
				case <-subCtx.Done():
					return
				}
			}
		}
	})

	return msgCh, nil
}

// Ack 确认消息已处理.
func (s *Subscriber) Ack(_ context.Context, msg *pubsub.Message) error {
	if msg == nil {
		return pubsub.ErrNilMessage
	}
	d, ok := msg.Metadata["delivery"].(amqp.Delivery)
	if !ok {
		return pubsub.ErrAckFailed
	}
	if err := d.Ack(false); err != nil {
		return fmt.Errorf("%w: %v", pubsub.ErrAckFailed, err)
	}
	return nil
}

// Nack 拒绝消息并重新入队.
func (s *Subscriber) Nack(_ context.Context, msg *pubsub.Message) error {
	if msg == nil {
		return pubsub.ErrNilMessage
	}
	d, ok := msg.Metadata["delivery"].(amqp.Delivery)
	if !ok {
		return pubsub.ErrNackFailed
	}
	if err := d.Nack(false, true); err != nil {
		return fmt.Errorf("%w: %v", pubsub.ErrNackFailed, err)
	}
	return nil
}

// Close 关闭 Subscriber. 幂等.
func (s *Subscriber) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	s.mu.Lock()
	for _, cancel := range s.cancels {
		cancel()
	}
	s.cancels = nil
	s.mu.Unlock()
	s.wg.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ch != nil {
		s.ch.Close()
	}
	return s.conn.Close()
}
