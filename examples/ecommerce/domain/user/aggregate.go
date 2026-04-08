// Package user 用户领域模型.
package user

import (
	"time"

	"github.com/Tsukikage7/servex/domain"
)

// User 用户聚合根.
type User struct {
	domain.AggregateRoot[uint64]
	username     string
	email        string
	passwordHash string
	createdAt    time.Time
	updatedAt    time.Time
}

// NewUser 创建用户聚合.
func NewUser(id uint64, username, email, passwordHash string) *User {
	u := &User{
		AggregateRoot: domain.NewAggregateRoot(id),
		username:      username,
		email:         email,
		passwordHash:  passwordHash,
		createdAt:     time.Now(),
		updatedAt:     time.Now(),
	}
	u.RaiseEvent(NewUserCreatedEvent(id, username, email))
	return u
}

// ReconstructUser 从持久化数据重建用户聚合（不触发事件）.
func ReconstructUser(id uint64, username, email, passwordHash string, createdAt, updatedAt time.Time) *User {
	return &User{
		AggregateRoot: domain.NewAggregateRoot(id),
		username:      username,
		email:         email,
		passwordHash:  passwordHash,
		createdAt:     createdAt,
		updatedAt:     updatedAt,
	}
}

// Username 返回用户名.
func (u *User) Username() string { return u.username }

// Email 返回邮箱.
func (u *User) Email() string { return u.email }

// PasswordHash 返回密码哈希.
func (u *User) PasswordHash() string { return u.passwordHash }

// CreatedAt 返回创建时间.
func (u *User) CreatedAt() time.Time { return u.createdAt }

// UpdatedAt 返回更新时间.
func (u *User) UpdatedAt() time.Time { return u.updatedAt }

// Update 更新用户信息.
func (u *User) Update(username, email string) {
	u.username = username
	u.email = email
	u.updatedAt = time.Now()
	u.RaiseEvent(NewUserUpdatedEvent(u.ID(), username, email))
}
