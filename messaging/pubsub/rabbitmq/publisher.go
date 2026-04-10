// Package rabbitmq 提供基于 RabbitMQ 的 pubsub.Publisher 和 pubsub.Subscriber 实现.
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

// Publisher 通过 RabbitMQ 发布消息.
type Publisher struct {
	url    string
	conn   *amqp.Connection
	ch     *amqp.Channel
	mu     sync.Mutex
	closed atomic.Bool
	opts   publisherOptions

	confirms chan amqp.Confirmation
}

// NewPublisher 基于 AMQP URL 创建 Publisher.
func NewPublisher(url string, opts ...PublisherOption) (*Publisher, error) {
	if url == "" {
		return nil, errors.New("pubsub/rabbitmq: url 不能为空")
	}

	o := publisherOptions{
		exchangeType: "direct",
		durable:      true,
		confirm:      true,
	}
	for _, opt := range opts {
		opt(&o)
	}

	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("pubsub/rabbitmq: 连接失败: %w", err)
	}

	p := &Publisher{url: url, conn: conn, opts: o}
	if err := p.setupChannel(); err != nil {
		conn.Close()
		return nil, err
	}

	// 启动连接断开监听和自动重连
	go p.reconnectLoop()

	return p, nil
}

// reconnectLoop 监听连接关闭通知，自动重连（指数退避）.
func (p *Publisher) reconnectLoop() {
	for {
		p.mu.Lock()
		conn := p.conn
		p.mu.Unlock()

		if conn == nil || p.closed.Load() {
			return
		}

		notifyClose := conn.NotifyClose(make(chan *amqp.Error, 1))
		amqpErr, ok := <-notifyClose
		if !ok || p.closed.Load() {
			return
		}

		p.logf("连接断开: %v，开始重连", amqpErr)

		backoff := time.Second
		const maxBackoff = 30 * time.Second
		for !p.closed.Load() {
			newConn, err := amqp.Dial(p.url)
			if err != nil {
				p.logf("重连失败: %v，%v 后重试", err, backoff)
				time.Sleep(backoff)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}

			p.mu.Lock()
			p.conn = newConn
			p.mu.Unlock()

			if err := p.setupChannel(); err != nil {
				p.logf("重连后创建 channel 失败: %v", err)
				newConn.Close()
				continue
			}

			p.logf("重连成功")
			break
		}
	}
}

func (p *Publisher) logf(format string, args ...any) {
	if p.opts.logger != nil {
		p.opts.logger.Infof("[RabbitMQ/Publisher] "+format, args...)
	}
}

func (p *Publisher) setupChannel() error {
	ch, err := p.conn.Channel()
	if err != nil {
		return fmt.Errorf("pubsub/rabbitmq: 创建 channel 失败: %w", err)
	}

	if p.opts.exchange != "" {
		if err := ch.ExchangeDeclare(
			p.opts.exchange,
			p.opts.exchangeType,
			p.opts.durable,
			false, false, false, nil,
		); err != nil {
			ch.Close()
			return fmt.Errorf("pubsub/rabbitmq: 声明交换机失败: %w", err)
		}
	}

	var confirms chan amqp.Confirmation
	if p.opts.confirm {
		if err := ch.Confirm(false); err != nil {
			ch.Close()
			return fmt.Errorf("pubsub/rabbitmq: 开启发布确认失败: %w", err)
		}
		confirms = ch.NotifyPublish(make(chan amqp.Confirmation, 100))
	}

	p.mu.Lock()
	p.ch = ch
	p.confirms = confirms
	p.mu.Unlock()
	return nil
}

// Publish 发布一条或多条消息到指定 routing key（topic）.
func (p *Publisher) Publish(ctx context.Context, topic string, msgs ...*pubsub.Message) error {
	if p.closed.Load() {
		return pubsub.ErrClosed
	}
	if topic == "" {
		return pubsub.ErrEmptyTopic
	}
	if len(msgs) == 0 {
		return pubsub.ErrNoMessages
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	ch := p.ch
	confirms := p.confirms
	p.mu.Unlock()

	for _, msg := range msgs {
		if msg == nil {
			return pubsub.ErrNilMessage
		}

		pub := amqp.Publishing{
			ContentType:  "application/octet-stream",
			Body:         msg.Body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			MessageId:    string(msg.Key),
		}
		if len(msg.Headers) > 0 {
			pub.Headers = make(amqp.Table, len(msg.Headers))
			for k, v := range msg.Headers {
				pub.Headers[k] = v
			}
		}

		if err := ch.PublishWithContext(ctx, p.opts.exchange, topic, false, false, pub); err != nil {
			return fmt.Errorf("pubsub/rabbitmq: 发布失败: %w", err)
		}

		if p.opts.confirm && confirms != nil {
			select {
			case confirm := <-confirms:
				if !confirm.Ack {
					return errors.New("pubsub/rabbitmq: broker 拒绝消息")
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return nil
}

// Close 关闭 Publisher. 幂等，多次调用不报错.
func (p *Publisher) Close() error {
	if p.closed.Swap(true) {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ch != nil {
		p.ch.Close()
	}
	return p.conn.Close()
}
