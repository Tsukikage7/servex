package domain

import "time"

// DomainEvent 领域事件接口.
type DomainEvent interface {
	EventName() string
	OccurredAt() time.Time
}

// BaseEvent 领域事件基类.
type BaseEvent struct {
	name       string
	occurredAt time.Time
}

// NewBaseEvent 创建领域事件.
// 可选参数 occurredAt 用于指定事件发生时间，默认为 time.Now().
func NewBaseEvent(name string, occurredAt ...time.Time) BaseEvent {
	t := time.Now()
	if len(occurredAt) > 0 {
		t = occurredAt[0]
	}
	return BaseEvent{name: name, occurredAt: t}
}

func (e BaseEvent) EventName() string     { return e.name }
func (e BaseEvent) OccurredAt() time.Time { return e.occurredAt }
