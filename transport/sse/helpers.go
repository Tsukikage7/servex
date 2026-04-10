package sse

import (
	"encoding/json"
	"fmt"
	"sync"
)

// NewEvent 创建新事件.
func NewEvent(eventType string, data []byte) *Event {
	return &Event{
		Event: eventType,
		Data:  data,
	}
}

// NewEventWithID 创建带 ID 的事件.
func NewEventWithID(id, eventType string, data []byte) *Event {
	return &Event{
		ID:    id,
		Event: eventType,
		Data:  data,
	}
}

// NewJSONEvent 创建 JSON 数据事件.
func NewJSONEvent(eventType string, data any) (*Event, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return &Event{
		Event: eventType,
		Data:  bytes,
	}, nil
}

// MustNewJSONEvent 创建 JSON 数据事件，失败时 panic.
func MustNewJSONEvent(eventType string, data any) *Event {
	event, err := NewJSONEvent(eventType, data)
	if err != nil {
		panic(err)
	}
	return event
}

// NewTextEvent 创建文本事件.
func NewTextEvent(eventType, text string) *Event {
	return &Event{
		Event: eventType,
		Data:  []byte(text),
	}
}

// NewMessageEvent 创建简单消息事件.
func NewMessageEvent(message string) *Event {
	return &Event{
		Event: "message",
		Data:  []byte(message),
	}
}

// EventBuilder 事件构建器.
type EventBuilder struct {
	event *Event
}

// NewBuilder 创建事件构建器.
func NewBuilder() *EventBuilder {
	return &EventBuilder{
		event: &Event{},
	}
}

// ID 设置事件 ID.
func (b *EventBuilder) ID(id string) *EventBuilder {
	b.event.ID = id
	return b
}

// Event 设置事件类型.
func (b *EventBuilder) Event(event string) *EventBuilder {
	b.event.Event = event
	return b
}

// Data 设置事件数据.
func (b *EventBuilder) Data(data []byte) *EventBuilder {
	b.event.Data = data
	return b
}

// Text 设置文本数据.
func (b *EventBuilder) Text(text string) *EventBuilder {
	b.event.Data = []byte(text)
	return b
}

// JSON 设置 JSON 数据.
func (b *EventBuilder) JSON(data any) *EventBuilder {
	bytes, err := json.Marshal(data)
	if err != nil {
		b.event.Data = []byte(fmt.Sprintf(`{"error":"%s"}`, err.Error()))
	} else {
		b.event.Data = bytes
	}
	return b
}

// Retry 设置重试间隔.
func (b *EventBuilder) Retry(ms int) *EventBuilder {
	b.event.Retry = ms
	return b
}

// Build 构建事件.
func (b *EventBuilder) Build() *Event {
	return b.event
}

// Broker 事件代理，支持主题订阅.
type Broker struct {
	server Server
	mu     sync.RWMutex                    // 保护 topics 的并发访问
	topics map[string]map[string]Client // topic -> clientID -> client
}

// NewBroker 创建事件代理.
func NewBroker(server Server) *Broker {
	return &Broker{
		server: server,
		topics: make(map[string]map[string]Client),
	}
}

// Subscribe 订阅主题.
func (b *Broker) Subscribe(clientID, topic string) error {
	client, ok := b.server.Client(clientID)
	if !ok {
		return ErrClientNotFound
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.topics[topic]; !ok {
		b.topics[topic] = make(map[string]Client)
	}
	b.topics[topic][clientID] = client
	return nil
}

// Unsubscribe 取消订阅.
func (b *Broker) Unsubscribe(clientID, topic string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if clients, ok := b.topics[topic]; ok {
		delete(clients, clientID)
		if len(clients) == 0 {
			delete(b.topics, topic)
		}
	}
}

// UnsubscribeAll 取消所有订阅.
func (b *Broker) UnsubscribeAll(clientID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for topic := range b.topics {
		delete(b.topics[topic], clientID)
		if len(b.topics[topic]) == 0 {
			delete(b.topics, topic)
		}
	}
}

// Publish 发布事件到主题.
func (b *Broker) Publish(topic string, event *Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if clients, ok := b.topics[topic]; ok {
		for _, client := range clients {
			_ = client.Send(event)
		}
	}
}

// Subscribers 返回主题订阅者数量.
func (b *Broker) Subscribers(topic string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if clients, ok := b.topics[topic]; ok {
		return len(clients)
	}
	return 0
}
