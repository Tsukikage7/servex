// Package event 提供轻量进程内事件总线，支持优先级、异步和通配符.
package event

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// ErrBusClosed 事件总线已关闭.
var ErrBusClosed = errors.New("event: bus is closed")

// Event 事件.
type Event struct {
	Name      string
	Payload   any
	Timestamp time.Time
}

// Handler 事件处理器.
type Handler func(ctx context.Context, evt *Event) error

// Bus 事件总线.
type Bus interface {
	// Publish 发布事件.
	Publish(ctx context.Context, name string, payload any) error
	// Subscribe 订阅事件（支持通配符：user.* 匹配 user.created, user.deleted）.
	Subscribe(pattern string, handler Handler, opts ...SubOption)
	// Unsubscribe 取消订阅.
	Unsubscribe(pattern string)
	// Close 关闭总线.
	Close() error
}

// SubOption 订阅选项.
type SubOption func(*subOptions)

type subOptions struct {
	priority int
	async    bool
}

// WithPriority 设置优先级，数字越小越先执行，默认 0.
func WithPriority(p int) SubOption {
	return func(o *subOptions) {
		o.priority = p
	}
}

// WithAsync 设置异步执行，默认 false.
func WithAsync(async bool) SubOption {
	return func(o *subOptions) {
		o.async = async
	}
}

// Option 总线选项.
type Option func(*bus)

// WithBufferSize 设置异步队列大小，默认 1024.
func WithBufferSize(n int) Option {
	return func(b *bus) {
		b.bufferSize = n
	}
}

// WithErrorHandler 设置错误处理函数.
func WithErrorHandler(fn func(err error)) Option {
	return func(b *bus) {
		b.errorHandler = fn
	}
}

type subscriber struct {
	handler  Handler
	priority int
	async    bool
}

// bus 事件总线实现.
type bus struct {
	mu           sync.RWMutex
	subscribers  map[string][]subscriber
	closed       bool
	bufferSize   int
	asyncCh      chan asyncTask
	errorHandler func(err error)
	wg           sync.WaitGroup
	closeOnce    sync.Once
}

type asyncTask struct {
	ctx     context.Context
	handler Handler
	event   *Event
}

// New 创建事件总线.
func New(opts ...Option) Bus {
	b := &bus{
		subscribers: make(map[string][]subscriber),
		bufferSize:  1024,
		errorHandler: func(err error) {
			// 默认通过标准日志输出错误，避免静默丢弃事件.
			log.Printf("[event] %v", err)
		},
	}
	for _, opt := range opts {
		opt(b)
	}

	b.asyncCh = make(chan asyncTask, b.bufferSize)
	b.wg.Add(1)
	go b.processAsync()

	return b
}

// processAsync 处理异步事件.
func (b *bus) processAsync() {
	defer b.wg.Done()
	for task := range b.asyncCh {
		if err := task.handler(task.ctx, task.event); err != nil {
			b.errorHandler(err)
		}
	}
}

// Publish 发布事件.
func (b *bus) Publish(ctx context.Context, name string, payload any) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ErrBusClosed
	}

	evt := &Event{
		Name:      name,
		Payload:   payload,
		Timestamp: time.Now(),
	}

	// 收集匹配的订阅者
	var matched []subscriber
	for pattern, subs := range b.subscribers {
		if matchPattern(pattern, name) {
			matched = append(matched, subs...)
		}
	}
	b.mu.RUnlock()

	// 按优先级排序
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].priority < matched[j].priority
	})

	// 执行处理器
	for _, sub := range matched {
		if sub.async {
			// 异步任务使用不可取消的 ctx，防止调用方 ctx 取消导致异步处理器无法执行
			asyncCtx := context.WithoutCancel(ctx)
			select {
			case b.asyncCh <- asyncTask{ctx: asyncCtx, handler: sub.handler, event: evt}:
			default:
				b.errorHandler(fmt.Errorf("event: 异步缓冲区已满，丢弃事件 %q", name))
			}
		} else {
			if err := sub.handler(ctx, evt); err != nil {
				return err
			}
		}
	}

	return nil
}

// Subscribe 订阅事件.
func (b *bus) Subscribe(pattern string, handler Handler, opts ...SubOption) {
	o := subOptions{}
	for _, opt := range opts {
		opt(&o)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	sub := subscriber{
		handler:  handler,
		priority: o.priority,
		async:    o.async,
	}
	b.subscribers[pattern] = append(b.subscribers[pattern], sub)
}

// Unsubscribe 取消订阅.
func (b *bus) Unsubscribe(pattern string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subscribers, pattern)
}

// Close 关闭事件总线.
func (b *bus) Close() error {
	b.closeOnce.Do(func() {
		b.mu.Lock()
		b.closed = true
		b.mu.Unlock()
		close(b.asyncCh)
	})
	b.wg.Wait()
	return nil
}

// matchPattern 匹配事件名称和模式.
// 支持通配符：
//   - "*" 匹配所有事件
//   - "user.*" 匹配 "user.created", "user.deleted" 等
//   - "user.created" 精确匹配
func matchPattern(pattern, name string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == name {
		return true
	}
	// 通配符匹配：a.* 匹配 a.b 但不匹配 a.b.c
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		if strings.HasPrefix(name, prefix+".") {
			// 确保 * 只匹配一层
			rest := strings.TrimPrefix(name, prefix+".")
			return !strings.Contains(rest, ".")
		}
	}
	return false
}
