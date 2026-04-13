// Package eventsourcing 实现事件溯源模式.
// 事件溯源通过存储聚合根上发生的所有事件来重建聚合状态，
// 而非直接存储当前状态。支持可选的快照机制加速聚合加载.
// 使用示例:
//
//	// 定义聚合
//	type Order struct {
//	    eventsourcing.BaseAggregate
//	    status string
//	}
//	func NewOrder(id string) *Order {
//	    return &Order{BaseAggregate: eventsourcing.NewBaseAggregate(id, "Order")}
//	}
//	func (o *Order) ApplyEvent(event eventsourcing.Event) error {
//	    switch event.EventType {
//	    case "OrderCreated":
//	        o.status = "created"
//	    }
//	    return nil
//	}
//	// 使用仓库
//	store := esgorm.NewEventStore(db)  // import esgorm "github.com/Tsukikage7/servex/v2/domain/eventsourcing/gorm"
//	repo := eventsourcing.NewRepository(store, func() *Order {
//	    return &Order{BaseAggregate: eventsourcing.NewBaseAggregate("", "Order")}
//	})
package eventsourcing

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Event 持久化事件.
type Event struct {
	ID            string          `json:"id" gorm:"primaryKey"`
	AggregateID   string          `json:"aggregate_id" gorm:"index:idx_agg_ver,unique"`
	AggregateType string          `json:"aggregate_type" gorm:"index:idx_agg_ver,unique"`
	Version       int64           `json:"version" gorm:"index:idx_agg_ver,unique"`
	EventType     string          `json:"event_type"`
	Data          json.RawMessage `json:"data"`
	Metadata      json.RawMessage `json:"metadata,omitzero"`
	CreatedAt     time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 指定表名.
func (Event) TableName() string {
	return "events"
}

// Aggregate 事件溯源聚合根接口.
type Aggregate interface {
	// AggregateID 返回聚合根 ID.
	AggregateID() string
	// AggregateType 返回聚合根类型.
	AggregateType() string
	// Version 返回当前版本号.
	Version() int64
	// SetVersion 设置版本号（从快照恢复时使用）.
	SetVersion(v int64)
	// ApplyEvent 应用事件到聚合状态.
	ApplyEvent(event Event) error
	// UncommittedEvents 返回未提交的事件列表.
	UncommittedEvents() []Event
	// ClearUncommittedEvents 清除未提交的事件.
	ClearUncommittedEvents()
}

// BaseAggregate 可嵌入的基础聚合根.
// 提供 Aggregate 接口的基础实现，业务聚合只需嵌入此结构并实现 ApplyEvent 方法.
type BaseAggregate struct {
	id                string
	aggregateType     string
	version           int64
	uncommittedEvents []Event
}

// NewBaseAggregate 创建基础聚合根.
func NewBaseAggregate(id, aggregateType string) BaseAggregate {
	return BaseAggregate{
		id:            id,
		aggregateType: aggregateType,
	}
}

// AggregateID 返回聚合根 ID.
func (a *BaseAggregate) AggregateID() string { return a.id }

// AggregateType 返回聚合根类型.
func (a *BaseAggregate) AggregateType() string { return a.aggregateType }

// Version 返回当前版本号.
func (a *BaseAggregate) Version() int64 { return a.version }

// SetVersion 设置版本号.
func (a *BaseAggregate) SetVersion(v int64) { a.version = v }

// UncommittedEvents 返回未提交的事件列表.
func (a *BaseAggregate) UncommittedEvents() []Event { return a.uncommittedEvents }

// ClearUncommittedEvents 清除未提交的事件.
func (a *BaseAggregate) ClearUncommittedEvents() { a.uncommittedEvents = nil }

// RaiseEvent 发起事件.
// 将 data 序列化为 JSON，创建 Event 并调用 ApplyEvent，然后追加到未提交列表.
// 需要外部聚合实现 ApplyEvent 方法，因此 applier 作为参数传入.
func (a *BaseAggregate) RaiseEvent(applier func(Event) error, eventType string, data any) error {
	rawData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	oldVersion := a.version
	a.version++
	event := Event{
		ID:            uuid.New().String(),
		AggregateID:   a.id,
		AggregateType: a.aggregateType,
		Version:       a.version,
		EventType:     eventType,
		Data:          rawData,
	}

	// 使用 defer 恢复版本号，确保 applier panic 时版本号也能正确回滚
	success := false
	defer func() {
		if !success {
			a.version = oldVersion
		}
	}()

	if err := applier(event); err != nil {
		return err
	}
	success = true

	a.uncommittedEvents = append(a.uncommittedEvents, event)
	return nil
}

// Snapshot 聚合快照.
type Snapshot struct {
	AggregateID   string          `json:"aggregate_id" gorm:"primaryKey"`
	AggregateType string          `json:"aggregate_type"`
	Version       int64           `json:"version"`
	Data          json.RawMessage `json:"data"`
	CreatedAt     time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 指定表名.
func (Snapshot) TableName() string {
	return "snapshots"
}
