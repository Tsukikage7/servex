// Package kafka 提供基于 Kafka 的 jobqueue.Store 实现.
package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/IBM/sarama"

	"github.com/Tsukikage7/servex/v2/messaging/jobqueue"
)

// Store 基于 Kafka 的 jobqueue.Store 实现.
// 每个 queue 对应一个 Kafka topic.
type Store struct {
	client   sarama.Client
	producer sarama.SyncProducer
	mu       sync.Mutex
	opts     options
}

// NewStore 基于 sarama.Client 创建 Kafka Store.
func NewStore(client sarama.Client, opts ...Option) (*Store, error) {
	if client == nil {
		return nil, errors.New("jobqueue/kafka: client 不能为空")
	}
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		return nil, errors.Join(errors.New("jobqueue/kafka: 创建 producer 失败"), err)
	}
	return &Store{client: client, producer: producer, opts: o}, nil
}

func (s *Store) topicName(queue string) string {
	if s.opts.prefix != "" {
		return s.opts.prefix + "." + queue
	}
	return queue
}

func (s *Store) Enqueue(ctx context.Context, job *jobqueue.Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	msg := &sarama.ProducerMessage{
		Topic: s.topicName(job.Queue),
		Key:   sarama.StringEncoder(job.ID),
		Value: sarama.ByteEncoder(data),
	}
	_, _, err = s.producer.SendMessage(msg)
	return err
}

// Dequeue 不直接支持 — Kafka 场景下使用 consumer group 拉取.
// 返回 ErrDequeueTimeout 表示需要使用 consumer-based 的方式.
func (s *Store) Dequeue(_ context.Context, _ string) (*jobqueue.Job, error) {
	return nil, jobqueue.ErrDequeueTimeout
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
	return s.producer.Close()
}
