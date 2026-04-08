package user

// CreateUserCommand 创建用户命令.
type CreateUserCommand struct {
	Username string `json:"username" validate:"required,min=2,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6,max=128"`
}

// UpdateUserCommand 更新用户命令.
type UpdateUserCommand struct {
	ID       uint64 `json:"-"`
	Username string `json:"username" validate:"required,min=2,max=50"`
	Email    string `json:"email" validate:"required,email"`
}
