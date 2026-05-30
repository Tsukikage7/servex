package eventsourcing

import (
	"context"
)

// EventStore 事件存储接口.
type EventStore interface {
	// Save 批量保存事件.
	Save(ctx context.Context, events []Event) error
	// Load 从指定版本加载事件.
	Load(ctx context.Context, aggregateID string, fromVersion int64) ([]Event, error)
	// LoadAll 加载聚合的全部事件.
	LoadAll(ctx context.Context, aggregateID string) ([]Event, error)
}

// SnapshotStore 快照存储接口.
type SnapshotStore interface {
	// Save 保存快照upsert 语义.
	Save(ctx context.Context, snapshot Snapshot) error
	// Load 加载最新快照.
	Load(ctx context.Context, aggregateID string) (*Snapshot, error)
}
