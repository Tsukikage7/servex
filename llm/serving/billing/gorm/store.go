// Package billinggorm 提供基于 GORM 的计费存储实现.
package billinggorm

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/llm/serving/billing"
)

// gormStore 基于 GORM 的 Store 实现.
type gormStore struct {
	db *gorm.DB
}

// NewGORMStore 创建基于 GORM 的 Store，使用 usage_records 表.
func NewGORMStore(db *gorm.DB) billing.Store {
	return &gormStore{db: db}
}

func (s *gormStore) SaveRecord(ctx context.Context, record *billing.UsageRecord) error {
	return s.db.WithContext(ctx).Create(record).Error
}

func (s *gormStore) GetRecords(ctx context.Context, keyID string, from, to time.Time) ([]billing.UsageRecord, error) {
	var records []billing.UsageRecord
	err := s.db.WithContext(ctx).
		Where("key_id = ? AND created_at >= ? AND created_at <= ?", keyID, from, to).
		Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (s *gormStore) AutoMigrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&billing.UsageRecord{})
}
