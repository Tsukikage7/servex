// Package jobqueuex provides jobqueue-backed async notification dispatch.
package jobqueuex

import (
	"context"
	"encoding/json"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/Tsukikage7/servex/v2/messaging/jobqueue"
	"github.com/Tsukikage7/servex/v2/notify"
)

const defaultQueue = "notifications"

// Option 配置异步通知投递器.
type Option func(*Dispatcher)

// Dispatcher 将通知消息序列化后投入 jobqueue.
type Dispatcher struct {
	client         jobqueue.Client
	queue          string
	defaultChannel notify.Channel
	closed         atomic.Bool
}

// NewDispatcher 创建 jobqueue 异步通知投递器.
func NewDispatcher(client jobqueue.Client, opts ...Option) *Dispatcher {
	d := &Dispatcher{
		client: client,
		queue:  defaultQueue,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// WithQueue 设置通知任务队列名.
func WithQueue(queue string) Option {
	return func(d *Dispatcher) {
		if queue != "" {
			d.queue = queue
		}
	}
}

// WithDefaultChannel 设置消息未指定渠道时使用的默认渠道.
func WithDefaultChannel(channel notify.Channel) Option {
	return func(d *Dispatcher) {
		d.defaultChannel = channel
	}
}

// SendAsync 将消息序列化后投入 jobqueue.
func (d *Dispatcher) SendAsync(ctx context.Context, msg *notify.Message) error {
	if d.closed.Load() {
		return notify.ErrClosed
	}
	if d.client == nil {
		return notify.ErrJobQueueNotConfigured
	}

	message := msg
	if msg != nil && msg.Channel == "" && d.defaultChannel != "" {
		clone := *msg
		clone.Channel = d.defaultChannel
		message = &clone
	}
	if err := notify.ValidateMessage(message); err != nil {
		return err
	}

	payload, err := json.Marshal(message)
	if err != nil {
		return notify.ErrSerializeFailed.WithCause(err)
	}

	return d.client.Enqueue(ctx, &jobqueue.Job{
		ID:      uuid.New().String(),
		Queue:   d.queue,
		Type:    "notification." + string(message.Channel),
		Payload: payload,
	})
}

// Close 关闭异步通知投递器.
func (d *Dispatcher) Close() error {
	if d.closed.Swap(true) {
		return nil
	}
	if d.client == nil {
		return nil
	}
	return d.client.Close()
}
