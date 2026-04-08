package user

import "github.com/Tsukikage7/servex/domain"

const (
	// EventUserCreated 用户创建事件名.
	EventUserCreated = "user.created"
	// EventUserUpdated 用户更新事件名.
	EventUserUpdated = "user.updated"
)

// UserCreatedEvent 用户创建事件.
type UserCreatedEvent struct {
	domain.BaseEvent
	UserID   uint64
	Username string
	Email    string
}

// NewUserCreatedEvent 创建用户创建事件.
func NewUserCreatedEvent(userID uint64, username, email string) *UserCreatedEvent {
	return &UserCreatedEvent{
		BaseEvent: domain.NewBaseEvent(EventUserCreated),
		UserID:    userID,
		Username:  username,
		Email:     email,
	}
}

// UserUpdatedEvent 用户更新事件.
type UserUpdatedEvent struct {
	domain.BaseEvent
	UserID   uint64
	Username string
	Email    string
}

// NewUserUpdatedEvent 创建用户更新事件.
func NewUserUpdatedEvent(userID uint64, username, email string) *UserUpdatedEvent {
	return &UserUpdatedEvent{
		BaseEvent: domain.NewBaseEvent(EventUserUpdated),
		UserID:    userID,
		Username:  username,
		Email:     email,
	}
}
