// Package auditgorm 提供基于 GORM 的审计日志存储实现.
package auditgorm

import (
	"context"

	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/v2/bizx/audit"
)

type gormStore struct {
	db *gorm.DB
}

// NewGORMStore 创建基于 GORM 的审计日志存储.
func NewGORMStore(db *gorm.DB) audit.Store {
	return &gormStore{db: db}
}

func (s *gormStore) Save(ctx context.Context, entry *audit.Entry) error {
	return s.db.WithContext(ctx).Create(entry).Error
}

func (s *gormStore) Query(ctx context.Context, filter *audit.Filter) ([]audit.Entry, error) {
	query := s.db.WithContext(ctx).Model(&audit.Entry{})
	query = applyFilter(query, filter)

	var entries []audit.Entry
	err := query.Order("created_at DESC").Find(&entries).Error
	return entries, err
}

func (s *gormStore) AutoMigrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&audit.Entry{})
}

// applyFilter 应用过滤条件到 GORM 查询.
func applyFilter(query *gorm.DB, filter *audit.Filter) *gorm.DB {
	if filter == nil {
		return query
	}
	if filter.Actor != "" {
		query = query.Where("actor = ?", filter.Actor)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.Resource != "" {
		query = query.Where("resource = ?", filter.Resource)
	}
	if filter.ResourceID != "" {
		query = query.Where("resource_id = ?", filter.ResourceID)
	}
	if !filter.From.IsZero() {
		query = query.Where("created_at >= ?", filter.From)
	}
	if !filter.To.IsZero() {
		query = query.Where("created_at <= ?", filter.To)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	return query
}
