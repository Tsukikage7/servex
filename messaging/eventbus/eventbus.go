// Package eventbus 提供轻量级进程内事件总线，支持同步和异步事件分发.
package eventbus

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
)

// Event 事件接口，所有事件必须实现 Topic 方法.
type Event interface {
	// Topic 返回事件主题.
	Topic() string
}

// Handler 事件处理函数.
type Handler func(ctx context.Context, event Event) error

// Logger 日志接口.
type Logger interface {
	Printf(format string, args ...any)
}

// Option 配置选项函数.
type Option func(*options)

type options struct {
	logger       Logger
	asyncWorkers int
	errorHandler func(error)
}

func defaultOptions() *options {
	return &options{
		logger:       log.Default(),
		asyncWorkers: 4,
		errorHandler: func(error) {},
	}
}

// WithLogger 设置日志记录器.
func WithLogger(l Logger) Option {
	return func(o *options) {
		if l != nil {
			o.logger = l
		}
	}
}

// WithAsyncWorkers 设置异步工作协程数量.
func WithAsyncWorkers(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.asyncWorkers = n
		}
	}
}

// WithErrorHandler 设置错误处理回调.
func WithErrorHandler(fn func(error)) Option {
	return func(o *options) {
		if fn != nil {
			o.errorHandler = fn
		}
	}
}

// subscriber 订阅者条目.
type subscriber struct {
	id      uint64
	handler Handler
}

// Bus 进程内事件总线.
type Bus struct {
	opts *options

	mu          sync.RWMutex
	subscribers map[string][]subscriber // topic -> subscribers
	wildcards   []subscriber            // 通配符订阅者
	nextID      uint64

	asyncCh chan asyncTask
	wg      sync.WaitGroup
	closed  atomic.Bool
	closeMu sync.RWMutex // 保护 asyncCh 的发送与关闭操作
}

// asyncTask 异步任务.
type asyncTask struct {
	ctx     context.Context
	event   Event
	handler Handler
}

// New 创建事件总线实例.
func New(opts ...Option) *Bus {
	o := defaultOptions()
	for _, fn := range opts {
		fn(o)
	}

	b := &Bus{
		opts:        o,
		subscribers: make(map[string][]subscriber),
		asyncCh:     make(chan asyncTask, o.asyncWorkers*16),
	}

	// 启动异步工作协程.
	for range o.asyncWorkers {
		b.wg.Go(func() { b.worker() })
	}

	return b
}

// worker 异步工作协程.
func (b *Bus) worker() {
	for task := range b.asyncCh {
		if err := task.handler(task.ctx, task.event); err != nil {
			b.opts.errorHandler(err)
		}
	}
}

// Subscribe 订阅指定主题的事件，返回取消订阅函数.
func (b *Bus) Subscribe(topic string, handler Handler) (unsubscribe func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++

	b.subscribers[topic] = append(b.subscribers[topic], subscriber{id: id, handler: handler})

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		subs := b.subscribers[topic]
		for i, s := range subs {
			if s.id == id {
				b.subscribers[topic] = append(subs[:i], subs[i+1:]...)
				return
			}
		}
	}
}

// SubscribeAll 订阅所有主题的事件（通配符），返回取消订阅函数.
func (b *Bus) SubscribeAll(handler Handler) (unsubscribe func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++

	b.wildcards = append(b.wildcards, subscriber{id: id, handler: handler})

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		for i, s := range b.wildcards {
			if s.id == id {
				b.wildcards = append(b.wildcards[:i], b.wildcards[i+1:]...)
				return
			}
		}
	}
}

// collect 收集匹配事件主题的所有处理器.
func (b *Bus) collect(topic string) []Handler {
	b.mu.RLock()
	defer b.mu.RUnlock()

	subs := b.subscribers[topic]
	handlers := make([]Handler, 0, len(subs)+len(b.wildcards))
	for _, s := range subs {
		handlers = append(handlers, s.handler)
	}
	for _, s := range b.wildcards {
		handlers = append(handlers, s.handler)
	}
	return handlers
}

// Publish 同步发布事件，依次调用所有匹配的处理器.
func (b *Bus) Publish(ctx context.Context, event Event) error {
	if b.closed.Load() {
		return ErrBusClosed
	}

	for _, h := range b.collect(event.Topic()) {
		if err := h(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

// PublishAsync 异步发布事件，将事件分发给工作协程处理.
func (b *Bus) PublishAsync(ctx context.Context, event Event) {
	// 使用 closeMu 读锁防止与 Close 中的 channel 关闭竞态
	b.closeMu.RLock()
	defer b.closeMu.RUnlock()

	if b.closed.Load() {
		return
	}

	for _, h := range b.collect(event.Topic()) {
		b.asyncCh <- asyncTask{ctx: ctx, event: event, handler: h}
	}
}

// Close 关闭事件总线，等待所有异步处理完成.
func (b *Bus) Close() {
	if b.closed.CompareAndSwap(false, true) {
		// 获取写锁，确保所有 PublishAsync 的发送已完成
		b.closeMu.Lock()
		close(b.asyncCh)
		b.closeMu.Unlock()
		b.wg.Wait()
	}
}

// ErrBusClosed 事件总线已关闭错误.
var ErrBusClosed = busClosedError{}

// busClosedError 事件总线已关闭错误类型.
type busClosedError struct{}

func (busClosedError) Error() string { return "eventbus: bus is closed" }
