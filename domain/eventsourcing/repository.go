package eventsourcing

import (
	"context"
	"encoding/json"
	"log/slog"
)

// Repository 事件溯源聚合仓库.
// 泛型参数 T 必须实现 Aggregate 接口.
// 支持可选的快照存储以加速聚合加载.
type Repository[T Aggregate] struct {
	eventStore    EventStore
	snapshotStore SnapshotStore // 可选
	factory       func() T      // 创建空聚合的工厂函数
	snapshotEvery int64         // 每 N 个事件保存快照，0 表示不保存
	logger        *slog.Logger  // 可选，用于记录快照操作日志
}

// RepositoryOption 仓库配置选项.
type RepositoryOption[T Aggregate] func(*Repository[T])

// WithSnapshotStore 设置快照存储.
func WithSnapshotStore[T Aggregate](store SnapshotStore) RepositoryOption[T] {
	return func(r *Repository[T]) {
		r.snapshotStore = store
	}
}

// WithSnapshotEvery 设置快照间隔.
// 每保存 n 个事件后自动创建快照，0 表示不保存快照.
func WithSnapshotEvery[T Aggregate](n int64) RepositoryOption[T] {
	return func(r *Repository[T]) {
		r.snapshotEvery = n
	}
}

// WithLogger 设置日志记录器.
func WithLogger[T Aggregate](l *slog.Logger) RepositoryOption[T] {
	return func(r *Repository[T]) {
		r.logger = l
	}
}

// NewRepository 创建聚合仓库.
func NewRepository[T Aggregate](eventStore EventStore, factory func() T, opts ...RepositoryOption[T]) (*Repository[T], error) {
	if eventStore == nil {
		return nil, ErrNilEventStore
	}
	if factory == nil {
		return nil, ErrNilFactory
	}

	r := &Repository[T]{
		eventStore: eventStore,
		factory:    factory,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r, nil
}

// Save 保存聚合的未提交事件.
// 获取未提交事件列表，持久化到事件存储，
// 并在满足快照条件时自动保存快照.
func (r *Repository[T]) Save(ctx context.Context, aggregate T) error {
	events := aggregate.UncommittedEvents()
	if len(events) == 0 {
		return ErrNoEvents
	}

	if err := r.eventStore.Save(ctx, events); err != nil {
		return err
	}

	// 尝试保存快照（失败仅记录日志，不影响主流程）
	if r.snapshotStore != nil && r.snapshotEvery > 0 && aggregate.Version()%r.snapshotEvery == 0 {
		data, err := json.Marshal(aggregate)
		if err != nil {
			if r.logger != nil {
				r.logger.Warn("[EventSourcing] 快照序列化失败",
					"aggregate_id", aggregate.AggregateID(),
					"error", err,
				)
			}
		} else if err := r.snapshotStore.Save(ctx, Snapshot{
			AggregateID:   aggregate.AggregateID(),
			AggregateType: aggregate.AggregateType(),
			Version:       aggregate.Version(),
			Data:          data,
		}); err != nil {
			if r.logger != nil {
				r.logger.Warn("[EventSourcing] 快照保存失败",
					"aggregate_id", aggregate.AggregateID(),
					"version", aggregate.Version(),
					"error", err,
				)
			}
		}
	}

	aggregate.ClearUncommittedEvents()
	return nil
}

// Load 加载聚合.
// 加载流程:
//  1. 如果配置了快照存储，尝试加载快照
//  2. 通过工厂创建空聚合
//  3. 如果找到快照，反序列化到聚合并设置版本
//  4. 从快照版本（或 0）加载后续事件
//  5. 逐个应用事件
//  6. 返回聚合
func (r *Repository[T]) Load(ctx context.Context, aggregateID string) (T, error) {
	aggregate := r.factory()
	var fromVersion int64

	// 尝试从快照恢复（失败时 fallback 从头加载）
	if r.snapshotStore != nil {
		snapshot, err := r.snapshotStore.Load(ctx, aggregateID)
		if err != nil {
			if r.logger != nil {
				r.logger.Warn("[EventSourcing] 快照加载失败，将从头重放事件",
					"aggregate_id", aggregateID,
					"error", err,
				)
			}
			// fallback: 忽略快照错误，从头加载
		} else if snapshot != nil {
			if err := json.Unmarshal(snapshot.Data, aggregate); err != nil {
				if r.logger != nil {
					r.logger.Warn("[EventSourcing] 快照反序列化失败，将从头重放事件",
						"aggregate_id", aggregateID,
						"error", err,
					)
				}
				// fallback: 重建空聚合，从头加载
				aggregate = r.factory()
			} else {
				aggregate.SetVersion(snapshot.Version)
				fromVersion = snapshot.Version
			}
		}
	}

	// 加载快照之后的事件
	events, err := r.eventStore.Load(ctx, aggregateID, fromVersion)
	if err != nil {
		var zero T
		return zero, err
	}

	// 没有快照也没有事件，聚合不存在
	if fromVersion == 0 && len(events) == 0 {
		var zero T
		return zero, ErrAggregateNotFound
	}

	// 逐个应用事件
	for _, event := range events {
		if err := aggregate.ApplyEvent(event); err != nil {
			var zero T
			return zero, err
		}
		aggregate.SetVersion(event.Version)
	}

	return aggregate, nil
}
