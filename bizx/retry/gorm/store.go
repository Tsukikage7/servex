// Package retrygorm 提供基于 GORM 的重试任务存储实现.
package retrygorm

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/v2/bizx/retry"
)

type gormStore struct {
	db *gorm.DB
}

// NewGORMStore 创建基于 GORM 的任务存储.
func NewGORMStore(db *gorm.DB) retry.Store {
	return &gormStore{db: db}
}

func (s *gormStore) Save(ctx context.Context, task *retry.Task) error {
	return s.db.WithContext(ctx).Create(task).Error
}

func (s *gormStore) FetchPending(ctx context.Context, limit int) ([]retry.Task, error) {
	var tasks []retry.Task
	err := s.db.WithContext(ctx).
		Where("status = ? AND next_retry_at <= ?", retry.StatusPending, time.Now()).
		Order("next_retry_at ASC").
		Limit(limit).
		Find(&tasks).Error
	return tasks, err
}

func (s *gormStore) Update(ctx context.Context, task *retry.Task) error {
	return s.db.WithContext(ctx).Save(task).Error
}

func (s *gormStore) AutoMigrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&retry.Task{})
}
