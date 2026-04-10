// Package gorm 提供基于 GORM 的 webhook SubscriptionStore 实现.
package gorm

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/notify/webhook"
	"github.com/Tsukikage7/servex/storage/rdbms"
)

type subscriptionModel struct {
	ID       string `gorm:"primaryKey;size:36"`
	URL      string `gorm:"size:2048"`
	Secret   string `gorm:"size:255"`
	Events   string `gorm:"size:1024"` // 逗号分隔
	Metadata string `gorm:"type:text"` // JSON
}

// Store 基于 GORM 的 SubscriptionStore.
type Store struct {
	db        *gorm.DB
	tableName string
}

// Option Store 配置选项.
type Option func(*Store)

// WithTableName 设置数据库表名.
func WithTableName(name string) Option {
	return func(s *Store) { s.tableName = name }
}

// NewStore 创建基于 GORM 的 SubscriptionStore.
func NewStore(db rdbms.Database, opts ...Option) (*Store, error) {
	if db == nil {
		return nil, errors.New("webhook/store/gorm: db 不能为空")
	}
	gormDB := rdbms.AsGORM(db)
	if gormDB == nil {
		return nil, errors.New("webhook/store/gorm: 仅支持 GORM 数据库")
	}
	s := &Store{db: gormDB, tableName: "webhook_subscriptions"}
	for _, opt := range opts {
		opt(s)
	}
	if err := gormDB.Table(s.tableName).AutoMigrate(&subscriptionModel{}); err != nil {
		return nil, err
	}
	return s, nil
}

// Save 保存或更新订阅.
func (s *Store) Save(ctx context.Context, sub *webhook.Subscription) error {
	m := toModel(sub)
	return s.db.Table(s.tableName).WithContext(ctx).Save(&m).Error
}

// Delete 删除指定订阅.
func (s *Store) Delete(ctx context.Context, id string) error {
	return s.db.Table(s.tableName).WithContext(ctx).Where("id = ?", id).Delete(&subscriptionModel{}).Error
}

// ListByEvent 按事件类型查询订阅列表.
func (s *Store) ListByEvent(ctx context.Context, eventType string) ([]*webhook.Subscription, error) {
	// 转义 LIKE 通配符，防止用户输入的 % 和 _ 导致意外匹配.
	escaped := escapeLike(eventType)
	var models []subscriptionModel
	if err := s.db.Table(s.tableName).WithContext(ctx).
		Where("events = '' OR events LIKE ?", "%"+escaped+"%").
		Find(&models).Error; err != nil {
		return nil, err
	}
	subs := make([]*webhook.Subscription, len(models))
	for i := range models {
		subs[i] = fromModel(&models[i])
	}
	return subs, nil
}

// escapeLike 转义 SQL LIKE 通配符 % 和 _.
func escapeLike(s string) string {
	r := strings.NewReplacer("%", "\\%", "_", "\\_")
	return r.Replace(s)
}

// Get 按 ID 查询单个订阅.
func (s *Store) Get(ctx context.Context, id string) (*webhook.Subscription, error) {
	var m subscriptionModel
	if err := s.db.Table(s.tableName).WithContext(ctx).Where("id = ?", id).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, webhook.ErrNotFound
		}
		return nil, err
	}
	return fromModel(&m), nil
}

func toModel(sub *webhook.Subscription) *subscriptionModel {
	return &subscriptionModel{
		ID:     sub.ID,
		URL:    sub.URL,
		Secret: sub.Secret,
		Events: strings.Join(sub.Events, ","),
	}
}

func fromModel(m *subscriptionModel) *webhook.Subscription {
	var events []string
	if m.Events != "" {
		events = strings.Split(m.Events, ",")
	}
	return &webhook.Subscription{
		ID:     m.ID,
		URL:    m.URL,
		Secret: m.Secret,
		Events: events,
	}
}
