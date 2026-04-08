package user

import "time"

// GetUserQuery 查询单个用户.
type GetUserQuery struct {
	ID uint64
}

// ListUsersQuery 分页查询用户列表.
type ListUsersQuery struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

// UserView 用户视图对象（返回给外部调用方）.
type UserView struct {
	ID        uint64    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ToView 将聚合转换为视图对象.
func ToView(u *User) *UserView {
	return &UserView{
		ID:        u.ID(),
		Username:  u.Username(),
		Email:     u.Email(),
		CreatedAt: u.CreatedAt(),
		UpdatedAt: u.UpdatedAt(),
	}
}
