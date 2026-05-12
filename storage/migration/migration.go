// Package migration 提供 Go DSL 数据库迁移工具.
package migration

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Migration 单次迁移定义.
type Migration struct {
	// Version 迁移版本号，通常使用时间戳.
	Version int64
	// Description 迁移描述.
	Description string
	// Up 升级函数，在事务中执行.
	Up func(tx *gorm.DB) error
	// Down 降级函数，在事务中执行.
	Down func(tx *gorm.DB) error
}

// Registry 迁移注册表（并发安全）.
type Registry struct {
	mu         sync.Mutex
	migrations []Migration
}

// NewRegistry 创建迁移注册表.
func NewRegistry() *Registry {
	return &Registry{}
}

// Add 添加一个迁移，支持链式调用.
func (r *Registry) Add(m Migration) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.migrations = append(r.migrations, m)
	return r
}

// Migrations 按 Version 排序返回所有迁移.
func (r *Registry) Migrations() []Migration {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]Migration, len(r.migrations))
	copy(result, r.migrations)
	slices.SortFunc(result, func(a, b Migration) int {
		return cmp.Compare(a.Version, b.Version)
	})
	return result
}

// MigrationStatus 迁移状态.
type MigrationStatus struct {
	// Version 迁移版本号.
	Version int64
	// Description 迁移描述.
	Description string
	// Applied 是否已应用.
	Applied bool
	// AppliedAt 应用时间.
	AppliedAt *time.Time
}

// Runner 迁移执行器接口.
type Runner interface {
	// Up 执行所有未应用的迁移.
	Up(ctx context.Context) error
	// UpTo 执行迁移到指定版本（含）.
	UpTo(ctx context.Context, version int64) error
	// Down 回滚最后一次迁移.
	Down(ctx context.Context) error
	// DownTo 回滚到指定版本（不含）.
	DownTo(ctx context.Context, version int64) error
	// Status 获取所有迁移状态.
	Status(ctx context.Context) ([]MigrationStatus, error)
	// CurrentVersion 获取当前版本号.
	CurrentVersion(ctx context.Context) (int64, error)
}

// NewRunner 创建迁移执行器.
func NewRunner(db *gorm.DB, registry *Registry, log logger.Logger) (Runner, error) {
	if db == nil {
		return nil, ErrNilDB
	}
	if registry == nil {
		return nil, ErrNilRegistry
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	store := newGORMStore(db)

	r := &runner{
		db:       db,
		registry: registry,
		store:    store,
		log:      logger.WithComponent(log, "Migration"),
	}
	r.ensureMigrated = sync.OnceValue(func() error {
		return store.AutoMigrate(context.Background())
	})
	return r, nil
}
