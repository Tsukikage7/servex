// Package rbacgorm 提供基于 GORM 的 RBAC 角色存储实现.
package rbacgorm

import (
	"context"

	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/auth/rbac"
)

// gormStore 基于 GORM 的角色存储.
type gormStore struct {
	db *gorm.DB
}

// NewGORMStore 创建基于 GORM 的角色存储.
func NewGORMStore(db *gorm.DB) rbac.Store {
	return &gormStore{db: db}
}

func (s *gormStore) SaveRole(ctx context.Context, role *rbac.Role) error {
	return s.db.WithContext(ctx).Save(role).Error
}

func (s *gormStore) GetRole(ctx context.Context, name string) (*rbac.Role, error) {
	var role rbac.Role
	err := s.db.WithContext(ctx).Where("name = ?", name).First(&role).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, rbac.ErrRoleNotFound
		}
		return nil, err
	}
	return &role, nil
}

func (s *gormStore) DeleteRole(ctx context.Context, name string) error {
	result := s.db.WithContext(ctx).Where("name = ?", name).Delete(&rbac.Role{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return rbac.ErrRoleNotFound
	}
	return nil
}

func (s *gormStore) ListRoles(ctx context.Context) ([]*rbac.Role, error) {
	var roles []*rbac.Role
	err := s.db.WithContext(ctx).Find(&roles).Error
	return roles, err
}

func (s *gormStore) AssignRole(ctx context.Context, userID, roleName string) error {
	return s.db.WithContext(ctx).Save(&rbac.UserRole{
		UserID:   userID,
		RoleName: roleName,
	}).Error
}

func (s *gormStore) RevokeRole(ctx context.Context, userID, roleName string) error {
	return s.db.WithContext(ctx).Where("user_id = ? AND role_name = ?", userID, roleName).
		Delete(&rbac.UserRole{}).Error
}

func (s *gormStore) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	var roles []rbac.UserRole
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&roles).Error
	if err != nil {
		return nil, err
	}
	names := make([]string, len(roles))
	for i, r := range roles {
		names[i] = r.RoleName
	}
	return names, nil
}

func (s *gormStore) AutoMigrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&rbac.Role{}, &rbac.UserRole{})
}
