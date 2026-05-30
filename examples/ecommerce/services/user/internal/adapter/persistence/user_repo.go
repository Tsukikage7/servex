package persistence

import (
	"context"
	"errors"

	"gorm.io/gorm"

	domainUser "github.com/Tsukikage7/servex/v2/examples/ecommerce/domain/user"
)

// UserRepository 基于 GORM 的用户仓储实现.
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建用户仓储.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create 创建用户.
func (r *UserRepository) Create(ctx context.Context, user *domainUser.User) error {
	po := FromAggregate(user)
	return r.db.WithContext(ctx).Create(po).Error
}

// GetByID 根据 ID 查询用户.
func (r *UserRepository) GetByID(ctx context.Context, id uint64) (*domainUser.User, error) {
	var po UserPO
	if err := r.db.WithContext(ctx).First(&po, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainUser.ErrNotFound
		}
		return nil, err
	}
	return po.ToAggregate(), nil
}

// GetByEmail 根据邮箱查询用户.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domainUser.User, error) {
	var po UserPO
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&po).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainUser.ErrNotFound
		}
		return nil, err
	}
	return po.ToAggregate(), nil
}

// Update 更新用户.
func (r *UserRepository) Update(ctx context.Context, user *domainUser.User) error {
	po := FromAggregate(user)
	return r.db.WithContext(ctx).Save(po).Error
}

// List 按条件分页查询用户列表.
func (r *UserRepository) List(ctx context.Context, filter domainUser.Filter) ([]*domainUser.User, int64, error) {
	query := r.db.WithContext(ctx).Model(&UserPO{})

	if filter.Username != "" {
		query = query.Where("username LIKE ?", "%"+filter.Username+"%")
	}
	if filter.Email != "" {
		query = query.Where("email LIKE ?", "%"+filter.Email+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if filter.Limit <= 0 {
		filter.Limit = 20
	}

	var pos []UserPO
	if err := query.Offset(filter.Offset).Limit(filter.Limit).Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}

	users := make([]*domainUser.User, 0, len(pos))
	for _, po := range pos {
		users = append(users, po.ToAggregate())
	}
	return users, total, nil
}
