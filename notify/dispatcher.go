package notify

import (
	"context"
	"sync"
	"sync/atomic"
)

// Dispatcher 多渠道消息分发器.
type Dispatcher struct {
	opts    dispatcherOptions
	senders map[Channel]Sender
	mu      sync.RWMutex
	closed  atomic.Bool
}

// NewDispatcher 创建消息分发器.
func NewDispatcher(opts ...Option) *Dispatcher {
	var o dispatcherOptions
	for _, opt := range opts {
		opt(&o)
	}
	return &Dispatcher{opts: o, senders: make(map[Channel]Sender)}
}

// Register 注册指定渠道的发送器.
func (d *Dispatcher) Register(sender Sender) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.senders[sender.Channel()] = sender
}

// Send 同步发送消息.
func (d *Dispatcher) Send(ctx context.Context, msg *Message) (*Result, error) {
	if d.closed.Load() {
		return nil, ErrClosed
	}
	if msg != nil && msg.Channel == "" && d.opts.defaultChannel != "" {
		msg.Channel = d.opts.defaultChannel
	}
	if err := ValidateMessage(msg); err != nil {
		return nil, err
	}

	if msg.TemplateID != "" && d.opts.templateEngine != nil {
		rendered, err := d.opts.templateEngine.Render(msg.TemplateID, msg.TemplateData)
		if err != nil {
			return nil, err
		}
		msg.Body = rendered
	}

	d.mu.RLock()
	sender, ok := d.senders[msg.Channel]
	d.mu.RUnlock()

	if !ok {
		return nil, ErrNoSender
	}
	return sender.Send(ctx, msg)
}

// Broadcast 向多个渠道广播消息.
func (d *Dispatcher) Broadcast(ctx context.Context, channels []Channel, msg *Message) []*Result {
	results := make([]*Result, 0, len(channels))
	for _, ch := range channels {
		clone := *msg
		clone.Channel = ch
		result, err := d.Send(ctx, &clone)
		if err != nil {
			results = append(results, &Result{Channel: ch, Error: err})
		} else {
			results = append(results, result)
		}
	}
	return results
}

// Close 关闭分发器及所有已注册的发送器.
func (d *Dispatcher) Close() error {
	if d.closed.Swap(true) {
		return nil
	}
	// 使用写锁取出所有 senders，然后在锁外关闭，避免阻塞 Register
	d.mu.Lock()
	senders := make([]Sender, 0, len(d.senders))
	for _, sender := range d.senders {
		senders = append(senders, sender)
	}
	d.mu.Unlock()

	var firstErr error
	for _, sender := range senders {
		if err := sender.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
