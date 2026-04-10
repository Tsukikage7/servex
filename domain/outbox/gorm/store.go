// Package outboxgorm 提供基于 GORM 的 outbox.Store 实现.
package outboxgorm

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Tsukikage7/servex/domain/outbox"
	"github.com/Tsukikage7/servex/storage/rdbms"
)

// Store 基于 GORM 的 outbox.Store 实现.
type Store struct {
	db *gorm.DB
}

// 编译期接口合规检查.
var _ outbox.Store = (*Store)(nil)

// NewStore 从 rdbms.Database 创建 Store.
func NewStore(db rdbms.Database) *Store {
	return &Store{db: rdbms.AsGORM(db)}
}

// NewStoreFromDB 从 *gorm.DB 创建 Store.
func NewStoreFromDB(db *gorm.DB) *Store {
	return &Store{db: db}
}

// Save 保存消息.
// 若 ctx 中注入了事务（通过 outbox.InjectTx），则在该事务中保存；否则直接保存.
func (s *Store) Save(ctx context.Context, msgs ...*outbox.OutboxMessage) error {
	if len(msgs) == 0 {
		return nil
	}
	db := s.db
	if tx, ok := outbox.ExtractTx(ctx); ok {
		db = tx
	}
	return db.WithContext(ctx).Create(msgs).Error
}

// WithTx 在事务中执行 fn.
// fn 收到的 ctx 中已通过 outbox.InjectTx 注入了事务，可直接调用 Save.
func (s *Store) WithTx(ctx context.Context, fn outbox.TxFunc) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := outbox.InjectTx(ctx, tx)
		return fn(txCtx)
	})
}

// FetchPending 拉取待发送消息并原子标记为 Processing.
// 对支持行锁的数据库（MySQL/PostgreSQL）使用 SELECT FOR UPDATE SKIP LOCKED，
// SQLite 环境自动降级为普通 SELECT.
func (s *Store) FetchPending(ctx context.Context, limit int) ([]*outbox.OutboxMessage, error) {
	var msgs []*outbox.OutboxMessage

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where("status = ?", outbox.StatusPending).
			Order("id ASC").
			Limit(limit)

		if !s.isSQLite() {
			query = query.Clauses(clause.Locking{
				Strength: "UPDATE",
				Options:  "SKIP LOCKED",
			})
		}

		if err := query.Find(&msgs).Error; err != nil {
			return err
		}

		if len(msgs) == 0 {
			return nil
		}

		ids := make([]uint64, len(msgs))
		for i, m := range msgs {
			ids[i] = m.ID
		}

		return tx.Model(&outbox.OutboxMessage{}).
			Where("id IN ?", ids).
			Update("status", outbox.StatusProcessing).Error
	})

	return msgs, err
}

// MarkSent 批量标记消息为已发送.
func (s *Store) MarkSent(ctx context.Context, ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}
	now := time.Now()
	return s.db.WithContext(ctx).
		Model(&outbox.OutboxMessage{}).
		Where("id IN ?", ids).
		Updates(map[string]any{
			"status":  outbox.StatusSent,
			"sent_at": now,
		}).Error
}

// MarkFailed 标记消息发送失败，递增重试计数.
func (s *Store) MarkFailed(ctx context.Context, id uint64, errMsg string) error {
	return s.db.WithContext(ctx).
		Model(&outbox.OutboxMessage{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":      outbox.StatusFailed,
			"retry_count": gorm.Expr("retry_count + 1"),
			"last_error":  errMsg,
		}).Error
}

// ResetStale 将超时的 Processing 消息重置为 Pending.
// 仅重置 StatusProcessing 状态，StatusFailed 由重试逻辑单独处理，避免无限重试循环.
func (s *Store) ResetStale(ctx context.Context, staleDuration time.Duration) (int64, error) {
	threshold := time.Now().Add(-staleDuration)
	result := s.db.WithContext(ctx).
		Model(&outbox.OutboxMessage{}).
		Where("status = ? AND updated_at < ?", outbox.StatusProcessing, threshold).
		Updates(map[string]any{
			"status": outbox.StatusPending,
		})
	return result.RowsAffected, result.Error
}

// Cleanup 删除指定时间之前的已发送消息.
func (s *Store) Cleanup(ctx context.Context, before time.Time) (int64, error) {
	result := s.db.WithContext(ctx).
		Where("status = ? AND sent_at < ?", outbox.StatusSent, before).
		Delete(&outbox.OutboxMessage{})
	return result.RowsAffected, result.Error
}

// AutoMigrate 自动迁移 outbox_messages 表.
func (s *Store) AutoMigrate() error {
	return s.db.AutoMigrate(&outbox.OutboxMessage{})
}

// isSQLite 检测当前是否使用 SQLite.
func (s *Store) isSQLite() bool {
	return s.db.Dialector.Name() == "sqlite"
}
