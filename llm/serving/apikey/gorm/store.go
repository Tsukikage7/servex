// Package apikeygorm 提供基于 GORM 的 API Key 存储实现.
package apikeygorm

import (
	"context"

	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/v2/llm/serving/apikey"
)

// GORMStore 基于 GORM 的 Store 实现.
type GORMStore struct {
	db *gorm.DB
}

// NewGORMStore 创建基于 GORM 的 Store.
func NewGORMStore(db *gorm.DB) *GORMStore {
	return &GORMStore{db: db}
}

func (s *GORMStore) Save(ctx context.Context, key *apikey.Key) error {
	return s.db.WithContext(ctx).Create(key).Error
}

func (s *GORMStore) GetByHash(ctx context.Context, hashedKey string) (*apikey.Key, error) {
	var key apikey.Key
	if err := s.db.WithContext(ctx).Where("hashed_key = ?", hashedKey).First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (s *GORMStore) GetByID(ctx context.Context, id string) (*apikey.Key, error) {
	var key apikey.Key
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (s *GORMStore) List(ctx context.Context, ownerID string) ([]*apikey.Key, error) {
	var keys []*apikey.Key
	if err := s.db.WithContext(ctx).Where("owner_id = ?", ownerID).Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

func (s *GORMStore) Update(ctx context.Context, key *apikey.Key) error {
	return s.db.WithContext(ctx).Save(key).Error
}

func (s *GORMStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Where("id = ?", id).Delete(&apikey.Key{}).Error
}

func (s *GORMStore) AutoMigrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&apikey.Key{})
}
