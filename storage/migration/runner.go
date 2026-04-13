package migration

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// runner 迁移执行器实现.
type runner struct {
	db       *gorm.DB
	registry *Registry
	store    Store
	log      logger.Logger
	// ensureMigrated 确保迁移记录表已创建（只执行一次）.
	ensureMigrated func() error
}

// Up 执行所有未应用的迁移.
func (r *runner) Up(ctx context.Context) error {
	if err := r.ensureMigrated(); err != nil {
		return fmt.Errorf("migration: 创建迁移记录表失败: %w", err)
	}

	migrations := r.registry.Migrations()
	if len(migrations) == 0 {
		return ErrNoMigrations
	}

	applied, err := r.appliedSet(ctx)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}
		if err := r.applyUp(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

// UpTo 执行迁移到指定版本（含）.
func (r *runner) UpTo(ctx context.Context, version int64) error {
	if err := r.ensureMigrated(); err != nil {
		return fmt.Errorf("migration: 创建迁移记录表失败: %w", err)
	}

	migrations := r.registry.Migrations()
	if len(migrations) == 0 {
		return ErrNoMigrations
	}

	if !r.versionExists(migrations, version) {
		return ErrVersionNotFound
	}

	applied, err := r.appliedSet(ctx)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if m.Version > version {
			break
		}
		if applied[m.Version] {
			continue
		}
		if err := r.applyUp(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

// Down 回滚最后一次迁移.
func (r *runner) Down(ctx context.Context) error {
	if err := r.ensureMigrated(); err != nil {
		return fmt.Errorf("migration: 创建迁移记录表失败: %w", err)
	}

	lastVersion, err := r.CurrentVersion(ctx)
	if err != nil {
		return err
	}
	if lastVersion == 0 {
		return ErrNotApplied
	}

	m, ok := r.findMigration(lastVersion)
	if !ok {
		return ErrVersionNotFound
	}

	return r.applyDown(ctx, m)
}

// DownTo 回滚到指定版本（不含），即保留 target 版本.
func (r *runner) DownTo(ctx context.Context, version int64) error {
	if err := r.ensureMigrated(); err != nil {
		return fmt.Errorf("migration: 创建迁移记录表失败: %w", err)
	}

	appliedRecords, err := r.store.Applied(ctx)
	if err != nil {
		return fmt.Errorf("migration: 获取已应用记录失败: %w", err)
	}

	// 按版本降序排列，从最新开始回滚.
	slices.SortFunc(appliedRecords, func(a, b AppliedMigration) int {
		return cmp.Compare(b.Version, a.Version)
	})

	for _, rec := range appliedRecords {
		if rec.Version <= version {
			break
		}
		m, ok := r.findMigration(rec.Version)
		if !ok {
			return fmt.Errorf("migration: 版本 %d 在注册表中未找到: %w", rec.Version, ErrVersionNotFound)
		}
		if err := r.applyDown(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

// Status 获取所有迁移状态.
func (r *runner) Status(ctx context.Context) ([]MigrationStatus, error) {
	if err := r.ensureMigrated(); err != nil {
		return nil, fmt.Errorf("migration: 创建迁移记录表失败: %w", err)
	}

	appliedRecords, err := r.store.Applied(ctx)
	if err != nil {
		return nil, fmt.Errorf("migration: 获取已应用记录失败: %w", err)
	}

	appliedMap := make(map[int64]AppliedMigration, len(appliedRecords))
	for _, rec := range appliedRecords {
		appliedMap[rec.Version] = rec
	}

	migrations := r.registry.Migrations()
	statuses := make([]MigrationStatus, 0, len(migrations))
	for _, m := range migrations {
		s := MigrationStatus{
			Version:     m.Version,
			Description: m.Description,
		}
		if rec, ok := appliedMap[m.Version]; ok {
			s.Applied = true
			t := rec.AppliedAt
			s.AppliedAt = &t
		}
		statuses = append(statuses, s)
	}
	return statuses, nil
}

// CurrentVersion 获取当前最大已应用版本号.
func (r *runner) CurrentVersion(ctx context.Context) (int64, error) {
	if err := r.ensureMigrated(); err != nil {
		return 0, fmt.Errorf("migration: 创建迁移记录表失败: %w", err)
	}

	records, err := r.store.Applied(ctx)
	if err != nil {
		return 0, fmt.Errorf("migration: 获取已应用记录失败: %w", err)
	}
	if len(records) == 0 {
		return 0, nil
	}

	var maxVersion int64
	for _, rec := range records {
		if rec.Version > maxVersion {
			maxVersion = rec.Version
		}
	}
	return maxVersion, nil
}

// applyUp 在事务中执行升级迁移并记录.
func (r *runner) applyUp(ctx context.Context, m Migration) error {
	r.log.Infof("migration: 执行升级 version=%d description=%s", m.Version, m.Description)

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := m.Up(tx); err != nil {
			return fmt.Errorf("migration: 执行升级 version=%d 失败: %w", m.Version, err)
		}
		if err := r.store.Record(ctx, tx, m.Version, m.Description); err != nil {
			return fmt.Errorf("migration: 记录 version=%d 失败: %w", m.Version, err)
		}
		return nil
	})
}

// applyDown 在事务中执行降级迁移并移除记录.
func (r *runner) applyDown(ctx context.Context, m Migration) error {
	r.log.Infof("migration: 执行降级 version=%d description=%s", m.Version, m.Description)

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := m.Down(tx); err != nil {
			return fmt.Errorf("migration: 执行降级 version=%d 失败: %w", m.Version, err)
		}
		if err := r.store.Remove(ctx, tx, m.Version); err != nil {
			return fmt.Errorf("migration: 移除记录 version=%d 失败: %w", m.Version, err)
		}
		return nil
	})
}

// appliedSet 获取已应用版本的集合.
func (r *runner) appliedSet(ctx context.Context) (map[int64]bool, error) {
	records, err := r.store.Applied(ctx)
	if err != nil {
		return nil, fmt.Errorf("migration: 获取已应用记录失败: %w", err)
	}
	set := make(map[int64]bool, len(records))
	for _, rec := range records {
		set[rec.Version] = true
	}
	return set, nil
}

// findMigration 在注册表中查找指定版本的迁移.
func (r *runner) findMigration(version int64) (Migration, bool) {
	for _, m := range r.registry.Migrations() {
		if m.Version == version {
			return m, true
		}
	}
	return Migration{}, false
}

// versionExists 检查版本是否在迁移列表中.
func (r *runner) versionExists(migrations []Migration, version int64) bool {
	for _, m := range migrations {
		if m.Version == version {
			return true
		}
	}
	return false
}
