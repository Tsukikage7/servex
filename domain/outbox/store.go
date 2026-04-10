package outbox

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// txContextKey 事务 context key 类型（避免与其他包冲突）.
type txContextKey struct{}

// InjectTx 将 GORM 事务注入 context，供 Save 读取.
func InjectTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// ExtractTx 从 context 中提取 GORM 事务.
func ExtractTx(ctx context.Context) (*gorm.DB, bool) {
	tx, ok := ctx.Value(txContextKey{}).(*gorm.DB)
	return tx, ok && tx != nil
}

// TxFunc 在事务 context 中执行的函数.
type TxFunc func(ctx context.Context) error

// Store 发件箱存储接口.
type Store interface {
	// Save 保存消息.
	// 若 ctx 中通过 InjectTx 注入了事务，则在该事务中保存；否则直接保存.
	Save(ctx context.Context, msgs ...*OutboxMessage) error
	// WithTx 在事务中执行 fn，实现原子性语义.
	// fn 收到的 ctx 中已注入事务，可直接调用 Save.
	WithTx(ctx context.Context, fn TxFunc) error
	// FetchPending 拉取待发送消息并标记为 Processing.
	FetchPending(ctx context.Context, limit int) ([]*OutboxMessage, error)
	// MarkSent 批量标记消息为已发送.
	MarkSent(ctx context.Context, ids []uint64) error
	// MarkFailed 标记消息发送失败.
	MarkFailed(ctx context.Context, id uint64, errMsg string) error
	// ResetStale 重置超时的 Processing/Failed 消息为 Pending.
	ResetStale(ctx context.Context, staleDuration time.Duration) (int64, error)
	// Cleanup 清理指定时间之前的已发送消息.
	Cleanup(ctx context.Context, before time.Time) (int64, error)
	// AutoMigrate 自动迁移表结构.
	AutoMigrate() error
}
