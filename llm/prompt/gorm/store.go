// Package promptgorm 提供基于 GORM 的 prompt Registry 持久化实现.
//
// 表结构由 prompt.Version 自带的 GORM tag 定义`prompt_versions` 表，
// 复合主键 (Name, Version). 使用前调用 AutoMigrate 自动建表.
package promptgorm

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/v2/llm/prompt"
)

// gormStore 基于 GORM 的 Store 实现.
type gormStore struct {
	db *gorm.DB
}

// NewGORMStore 创建基于 GORM 的 Store，使用 prompt_versions 表.
//
// 注意：不会自动建表；调用方请在初始化阶段调用 AutoMigrate.
func NewGORMStore(db *gorm.DB) prompt.Store {
	return &gormStore{db: db}
}

// AutoMigrate 自动迁移 prompt_versions 表结构.
func AutoMigrate(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).AutoMigrate(&prompt.Version{})
}

// Save 保存或覆盖一条 Version 记录，按 (Name, Version) 主键判断.
//
// 注意：对已存在的主键，GORM 的 Save 语义会覆盖所有字段包括 CreatedAt.
// 若传入的 v.CreatedAt 为零值，本方法会把它补为当前时间；这对"新记录"语义正确,
// 但若调用方手动对已有版本再次 Save 且未保留原 CreatedAt，则该版本的 CreatedAt
// 会被重置为"本次写入时间". Registry 路径只用它写新版本version 号单调自增,
// 所以该行为在 Registry 内部无副作用；作为公开 API 使用时请调用方自行保留原 CreatedAt.
func (s *gormStore) Save(ctx context.Context, v *prompt.Version) error {
	if v == nil {
		return errors.New("prompt/gorm: nil version")
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now().UTC()
	}
	// 使用 Save 语义：按主键存在则 Update，不存在则 Insert.
	return s.db.WithContext(ctx).Save(v).Error
}

// LoadAll 按 Version 升序返回 name 下的所有记录；无记录返回 (nil, nil).
func (s *gormStore) LoadAll(ctx context.Context, name string) ([]prompt.Version, error) {
	var versions []prompt.Version
	err := s.db.WithContext(ctx).
		Where("name = ?", name).
		Order("version ASC").
		Find(&versions).Error
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, nil
	}
	return versions, nil
}

// LoadAllNames 返回所有已注册的 name去重、按字典序.
func (s *gormStore) LoadAllNames(ctx context.Context) ([]string, error) {
	var names []string
	err := s.db.WithContext(ctx).
		Model(&prompt.Version{}).
		Distinct("name").
		Order("name ASC").
		Pluck("name", &names).Error
	if err != nil {
		return nil, err
	}
	return names, nil
}

// UpdateFlags 更新指定 (name, version) 的 Active 与 Weight.
// 若记录不存在返回 prompt.ErrNotFound.
func (s *gormStore) UpdateFlags(ctx context.Context, name string, version int, active bool, weight int) error {
	res := s.db.WithContext(ctx).
		Model(&prompt.Version{}).
		Where("name = ? AND version = ?", name, version).
		Updates(map[string]any{
			"active": active,
			"weight": weight,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return prompt.ErrNotFound
	}
	return nil
}
