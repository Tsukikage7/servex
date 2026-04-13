// Package rabbitmq 提供基于 RabbitMQ 的 jobqueue.Store 实现.
package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/Tsukikage7/servex/v2/messaging/jobqueue"
)

// Store 基于 RabbitMQ 的 jobqueue.Store 实现.
type Store struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	mu      sync.Mutex
	opts    options
}

// NewStore 基于 AMQP 连接创建 RabbitMQ Store.
func NewStore(conn *amqp.Connection, opts ...Option) (*Store, error) {
	if conn == nil {
		return nil, errors.New("jobqueue/rabbitmq: connection 不能为空")
	}
	o := options{durable: true, prefetchCount: 1}
	for _, opt := range opts {
		opt(&o)
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	return &Store{conn: conn, channel: ch, opts: o}, nil
}

func (s *Store) queueName(queue string) string {
	if s.opts.prefix != "" {
		return s.opts.prefix + "." + queue
	}
	return queue
}

func (s *Store) Enqueue(ctx context.Context, job *jobqueue.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	qName := s.queueName(job.Queue)
	_, err := s.channel.QueueDeclare(qName, s.opts.durable, false, false, false, amqp.Table{
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": qName + ".dead",
	})
	if err != nil {
		return err
	}

	data, err := json.Marshal(job)
	if err != nil {
		return err
	}

	pub := amqp.Publishing{
		Body:         data,
		DeliveryMode: amqp.Persistent,
		Priority:     uint8(job.Priority),
		MessageId:    job.ID,
	}
	return s.channel.PublishWithContext(ctx, "", qName, false, false, pub)
}

func (s *Store) Dequeue(ctx context.Context, queue string) (*jobqueue.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	qName := s.queueName(queue)
	d, ok, err := s.channel.Get(qName, false)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, jobqueue.ErrDequeueTimeout
	}

	var job jobqueue.Job
	if err := json.Unmarshal(d.Body, &job); err != nil {
		d.Nack(false, false)
		return nil, err
	}
	d.Ack(false)
	return &job, nil
}

func (s *Store) MarkRunning(_ context.Context, _ string) error         { return nil }
func (s *Store) MarkFailed(_ context.Context, _ string, _ error) error { return nil }
func (s *Store) MarkDead(_ context.Context, _ string) error            { return nil }
func (s *Store) MarkDone(_ context.Context, _ string) error            { return nil }
func (s *Store) Requeue(ctx context.Context, job *jobqueue.Job) error {
	return s.Enqueue(ctx, job)
}
func (s *Store) ListDead(_ context.Context, _ string) ([]*jobqueue.Job, error) {
	return nil, nil
}

func (s *Store) Close() error {
	return s.channel.Close()
}
