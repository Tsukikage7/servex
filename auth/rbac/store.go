package rbac

import (
	"context"
	"sync"
	"time"
)

// UserRole 用户-角色关联.
type UserRole struct {
	UserID    string    `json:"user_id" gorm:"primaryKey;index:idx_user"`
	RoleName  string    `json:"role_name" gorm:"primaryKey"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// Store 角色存储接口.
type Store interface {
	SaveRole(ctx context.Context, role *Role) error
	GetRole(ctx context.Context, name string) (*Role, error)
	DeleteRole(ctx context.Context, name string) error
	ListRoles(ctx context.Context) ([]*Role, error)
	AssignRole(ctx context.Context, userID, roleName string) error
	RevokeRole(ctx context.Context, userID, roleName string) error
	GetUserRoles(ctx context.Context, userID string) ([]string, error)
	AutoMigrate(ctx context.Context) error
}

// memoryStore 基于内存的角色存储.
type memoryStore struct {
	mu        sync.RWMutex
	roles     map[string]*Role
	userRoles map[string]map[string]bool // userID -> roleName -> exists
}

// NewMemoryStore 创建基于内存的角色存储（用于测试）.
func NewMemoryStore() Store {
	return &memoryStore{
		roles:     make(map[string]*Role),
		userRoles: make(map[string]map[string]bool),
	}
}

func (s *memoryStore) SaveRole(_ context.Context, role *Role) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roles[role.Name] = role
	return nil
}

func (s *memoryStore) GetRole(_ context.Context, name string) (*Role, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	role, ok := s.roles[name]
	if !ok {
		return nil, ErrRoleNotFound
	}
	return role, nil
}

func (s *memoryStore) DeleteRole(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.roles[name]; !ok {
		return ErrRoleNotFound
	}
	delete(s.roles, name)
	return nil
}

func (s *memoryStore) ListRoles(_ context.Context) ([]*Role, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	roles := make([]*Role, 0, len(s.roles))
	for _, role := range s.roles {
		roles = append(roles, role)
	}
	return roles, nil
}

func (s *memoryStore) AssignRole(_ context.Context, userID, roleName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.userRoles[userID]; !ok {
		s.userRoles[userID] = make(map[string]bool)
	}
	s.userRoles[userID][roleName] = true
	return nil
}

func (s *memoryStore) RevokeRole(_ context.Context, userID, roleName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if roles, ok := s.userRoles[userID]; ok {
		delete(roles, roleName)
	}
	return nil
}

func (s *memoryStore) GetUserRoles(_ context.Context, userID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	roles, ok := s.userRoles[userID]
	if !ok {
		return nil, nil
	}
	names := make([]string, 0, len(roles))
	for name := range roles {
		names = append(names, name)
	}
	return names, nil
}

func (s *memoryStore) AutoMigrate(_ context.Context) error {
	return nil // 内存存储无需迁移
}
