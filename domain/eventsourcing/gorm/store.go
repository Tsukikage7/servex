// Package esgorm 提供基于 GORM 的事件溯源存储实现.
package esgorm

import (
	"context"
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Tsukikage7/servex/v2/domain/eventsourcing"
)

// EventStore 基于 GORM 的事件存储实现.
type EventStore struct {
	db *gorm.DB
}

// 编译期接口合规检查.
var _ eventsourcing.EventStore = (*EventStore)(nil)

// NewEventStore 创建 GORM 事件存储.
func NewEventStore(db *gorm.DB) *EventStore {
	return &EventStore{db: db}
}

// AutoMigrate 自动迁移 events 表.
func (s *EventStore) AutoMigrate() error {
	return s.db.AutoMigrate(&eventsourcing.Event{})
}

// Save 批量保存事件.
// 利用 (aggregate_id, aggregate_type, version) 唯一索引实现乐观并发控制，
// 当版本冲突时返回 ErrConcurrencyConflict.
func (s *EventStore) Save(ctx context.Context, events []eventsourcing.Event) error {
	if len(events) == 0 {
		return eventsourcing.ErrNoEvents
	}

	err := s.db.WithContext(ctx).Create(&events).Error
	if err != nil && isConcurrencyError(err) {
		return eventsourcing.ErrConcurrencyConflict
	}
	return err
}

// Load 从指定版本之后加载事件.
func (s *EventStore) Load(ctx context.Context, aggregateID string, fromVersion int64) ([]eventsourcing.Event, error) {
	var events []eventsourcing.Event
	err := s.db.WithContext(ctx).
		Where("aggregate_id = ? AND version > ?", aggregateID, fromVersion).
		Order("version ASC").
		Find(&events).Error
	return events, err
}

// LoadAll 加载聚合的全部事件.
func (s *EventStore) LoadAll(ctx context.Context, aggregateID string) ([]eventsourcing.Event, error) {
	var events []eventsourcing.Event
	err := s.db.WithContext(ctx).
		Where("aggregate_id = ?", aggregateID).
		Order("version ASC").
		Find(&events).Error
	return events, err
}

// isConcurrencyError 检测唯一约束冲突错误.
// 优先通过数据库驱动错误码精确判断MySQL 1062, PostgreSQL 23505，
// 回退到字符串匹配以兼容 SQLite 等其他数据库.
func isConcurrencyError(err error) bool {
	if err == nil {
		return false
	}

	// MySQL: 错误码 1062 (ER_DUP_ENTRY)
	if mysqlErr, ok := errors.AsType[*mysql.MySQLError](err); ok {
		return mysqlErr.Number == 1062
	}

	// PostgreSQL: 错误码 23505 (unique_violation)
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code == "23505"
	}

	// SQLite 及其他数据库: 回退到字符串匹配仅匹配 UNIQUE constraint 相关
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "duplicate entry") ||
		strings.Contains(msg, "unique_violation")
}

// SnapshotStore 基于 GORM 的快照存储实现.
type SnapshotStore struct {
	db *gorm.DB
}

// 编译期接口合规检查.
var _ eventsourcing.SnapshotStore = (*SnapshotStore)(nil)

// NewSnapshotStore 创建 GORM 快照存储.
func NewSnapshotStore(db *gorm.DB) *SnapshotStore {
	return &SnapshotStore{db: db}
}

// AutoMigrate 自动迁移 snapshots 表.
func (s *SnapshotStore) AutoMigrate() error {
	return s.db.AutoMigrate(&eventsourcing.Snapshot{})
}

// Save 保存快照upsert 语义.
// 使用 ON CONFLICT UPDATE 实现 upsert.
func (s *SnapshotStore) Save(ctx context.Context, snapshot eventsourcing.Snapshot) error {
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "aggregate_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"version", "data", "created_at"}),
		}).
		Create(&snapshot).Error
}

// Load 加载聚合的最新快照.
func (s *SnapshotStore) Load(ctx context.Context, aggregateID string) (*eventsourcing.Snapshot, error) {
	var snapshot eventsourcing.Snapshot
	err := s.db.WithContext(ctx).
		Where("aggregate_id = ?", aggregateID).
		First(&snapshot).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &snapshot, nil
}
