package domain

import (
	"context"
	"sync"
)

// EventHandler 事件处理器.
type EventHandler func(ctx context.Context, event DomainEvent) error

// EventBus 事件总线.
type EventBus struct {
	handlers map[string][]EventHandler
	mu       sync.RWMutex
}

// NewEventBus 创建事件总线.
func NewEventBus() *EventBus {
	return &EventBus{handlers: make(map[string][]EventHandler)}
}

// Subscribe 订阅事件.
func (b *EventBus) Subscribe(eventName string, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventName] = append(b.handlers[eventName], handler)
}

// SubscribeAll 订阅所有事件.
func (b *EventBus) SubscribeAll(handler EventHandler) {
	b.Subscribe("*", handler)
}

// Publish 发布事件.
// 注意：采用 fail-fast 策略，第一个处理器返回错误后即停止后续处理器调用.
func (b *EventBus) Publish(ctx context.Context, event DomainEvent) error {
	b.mu.RLock()
	// 复制 handlers 到新切片，避免 append 在 RLock 下复用底层数组导致竞态
	src := b.handlers[event.EventName()]
	wild := b.handlers["*"]
	handlers := make([]EventHandler, 0, len(src)+len(wild))
	handlers = append(handlers, src...)
	handlers = append(handlers, wild...)
	b.mu.RUnlock()

	for _, h := range handlers {
		if err := h(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

// Dispatch 从聚合发布所有事件并清除.
func (b *EventBus) Dispatch(ctx context.Context, events []DomainEvent, clear func()) error {
	for _, event := range events {
		if err := b.Publish(ctx, event); err != nil {
			return err
		}
	}
	clear()
	return nil
}
