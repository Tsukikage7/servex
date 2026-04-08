// Package persistence 用户持久化适配器.
package persistence

import (
	"time"

	domainUser "github.com/Tsukikage7/servex/examples/ecommerce/domain/user"
)

// UserPO 用户持久化对象.
type UserPO struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement:false"`
	Username     string    `gorm:"type:varchar(50);not null;uniqueIndex"`
	Email        string    `gorm:"type:varchar(100);not null;uniqueIndex"`
	PasswordHash string    `gorm:"type:varchar(255);not null"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

// TableName 指定表名.
func (UserPO) TableName() string { return "users" }

// ToAggregate 将持久化对象转换为领域聚合.
func (po *UserPO) ToAggregate() *domainUser.User {
	return domainUser.ReconstructUser(
		po.ID,
		po.Username,
		po.Email,
		po.PasswordHash,
		po.CreatedAt,
		po.UpdatedAt,
	)
}

// FromAggregate 将领域聚合转换为持久化对象.
func FromAggregate(u *domainUser.User) *UserPO {
	return &UserPO{
		ID:           u.ID(),
		Username:     u.Username(),
		Email:        u.Email(),
		PasswordHash: u.PasswordHash(),
		CreatedAt:    u.CreatedAt(),
		UpdatedAt:    u.UpdatedAt(),
	}
}
