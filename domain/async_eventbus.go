package domain

import (
	"context"
	"encoding/json"

	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
)

// EventConverter 领域事件到消息的转换器接口.
type EventConverter interface {
	// Convert 将领域事件转换为 pubsub.Message.
	Convert(event DomainEvent) (*pubsub.Message, error)
}

// AsyncEventBus 异步事件总线.
// 将领域事件通过 pubsub.Publisher 异步投递到消息队列，
// 同时保留同步内存处理器的能力.
type AsyncEventBus struct {
	publisher pubsub.Publisher
	converter EventConverter
	sync      *EventBus
}

// NewAsyncEventBus 创建异步事件总线.
func NewAsyncEventBus(publisher pubsub.Publisher, converter EventConverter) *AsyncEventBus {
	return &AsyncEventBus{
		publisher: publisher,
		converter: converter,
		sync:      NewEventBus(),
	}
}

// Subscribe 订阅同步内存处理器与同步 EventBus 行为一致.
func (b *AsyncEventBus) Subscribe(eventName string, handler EventHandler) {
	b.sync.Subscribe(eventName, handler)
}

// SubscribeAll 订阅所有事件的同步处理器.
func (b *AsyncEventBus) SubscribeAll(handler EventHandler) {
	b.sync.SubscribeAll(handler)
}

// Publish 发布领域事件.
// 处理顺序：
//  1. 同步调用内存处理器
//  2. 异步投递到消息队列
func (b *AsyncEventBus) Publish(ctx context.Context, event DomainEvent) error {
	// 1. 同步处理内存处理器
	if err := b.sync.Publish(ctx, event); err != nil {
		return err
	}

	// 2. 异步投递到消息队列
	msg, err := b.converter.Convert(event)
	if err != nil {
		return err
	}

	return b.publisher.Publish(ctx, msg.Topic, msg)
}

// Dispatch 从聚合发布所有事件并清除.
// 注意：采用 fail-fast 策略，任一事件发布失败时停止，已发布的事件不会回滚.
// 调用方应结合 outbox 模式保证最终一致性.
func (b *AsyncEventBus) Dispatch(ctx context.Context, events []DomainEvent, clear func()) error {
	for _, event := range events {
		if err := b.Publish(ctx, event); err != nil {
			return err
		}
	}
	clear()
	return nil
}

// JSONEventConverter 将领域事件序列化为 JSON 格式的消息转换器.
// Topic 使用 event.EventName()，Body 为 JSON 序列化结果.
type JSONEventConverter struct{}

// NewJSONEventConverter 创建 JSON 事件转换器.
func NewJSONEventConverter() *JSONEventConverter {
	return &JSONEventConverter{}
}

// Convert 将领域事件转换为 JSON 消息.
func (c *JSONEventConverter) Convert(event DomainEvent) (*pubsub.Message, error) {
	value, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	return &pubsub.Message{
		Topic: event.EventName(),
		Body:  value,
	}, nil
}

// 编译期接口合规检查.
var _ EventConverter = (*JSONEventConverter)(nil)
