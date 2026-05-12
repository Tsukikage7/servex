// Package user 用户应用服务.
package user

import (
	"context"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	servexJWT "github.com/Tsukikage7/servex/v2/auth/jwt"
	"github.com/Tsukikage7/servex/v2/domain"
	"github.com/Tsukikage7/servex/v2/errors"

	domainUser "github.com/Tsukikage7/servex/v2/examples/ecommerce/domain/user"
)

// 用户应用层错误.
var (
	ErrInvalidCredentials = errors.NewWithKind(50101, "user.invalid_credentials", "用户名或密码错误", errors.KindUnauthenticated)
	ErrHashPassword       = errors.NewWithKind(50102, "user.hash_password", "生成密码哈希失败", errors.KindInternal)
	ErrCreateUser         = errors.NewWithKind(50103, "user.create_failed", "创建用户失败", errors.KindInternal)
	ErrDispatchEvent      = errors.NewWithKind(50104, "user.dispatch_event", "分发领域事件失败", errors.KindInternal)
	ErrUpdateUser         = errors.NewWithKind(50105, "user.update_failed", "更新用户失败", errors.KindInternal)
	ErrGenerateToken      = errors.NewWithKind(50106, "user.generate_token", "生成令牌失败", errors.KindInternal)
)

// Service 用户应用服务.
type Service struct {
	repo     domainUser.Repository
	eventBus *domain.EventBus
	jwtSvc   *servexJWT.JWT
}

// NewService 创建用户应用服务.
func NewService(repo domainUser.Repository, eventBus *domain.EventBus, jwtSvc *servexJWT.JWT) *Service {
	return &Service{
		repo:     repo,
		eventBus: eventBus,
		jwtSvc:   jwtSvc,
	}
}

// Create 创建用户.
func (s *Service) Create(ctx context.Context, cmd domainUser.CreateUserCommand) (*domainUser.UserView, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(cmd.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, ErrHashPassword.WithCause(err)
	}

	// 使用时间戳作为简易 ID 生成（生产环境应使用分布式 ID）
	id := uint64(time.Now().UnixNano())

	user := domainUser.NewUser(id, cmd.Username, cmd.Email, string(hash))

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, ErrCreateUser.WithCause(err)
	}

	// 分发领域事件
	if err := s.eventBus.Dispatch(ctx, user.DomainEvents(), user.ClearDomainEvents); err != nil {
		return nil, ErrDispatchEvent.WithCause(err)
	}

	return domainUser.ToView(user), nil
}

// GetByID 根据 ID 查询用户.
func (s *Service) GetByID(ctx context.Context, id uint64) (*domainUser.UserView, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return domainUser.ToView(user), nil
}

// Update 更新用户信息.
func (s *Service) Update(ctx context.Context, cmd domainUser.UpdateUserCommand) (*domainUser.UserView, error) {
	user, err := s.repo.GetByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	user.Update(cmd.Username, cmd.Email)

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, ErrUpdateUser.WithCause(err)
	}

	if err := s.eventBus.Dispatch(ctx, user.DomainEvents(), user.ClearDomainEvents); err != nil {
		return nil, ErrDispatchEvent.WithCause(err)
	}

	return domainUser.ToView(user), nil
}

// List 分页查询用户列表.
func (s *Service) List(ctx context.Context, query domainUser.ListUsersQuery) ([]*domainUser.UserView, int64, error) {
	users, total, err := s.repo.List(ctx, domainUser.Filter{
		Offset: query.Offset,
		Limit:  query.Limit,
	})
	if err != nil {
		return nil, 0, err
	}

	views := make([]*domainUser.UserView, 0, len(users))
	for _, u := range users {
		views = append(views, domainUser.ToView(u))
	}
	return views, total, nil
}

// LoginRequest 登录请求.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse 登录响应.
type LoginResponse struct {
	Token string               `json:"token"`
	User  *domainUser.UserView `json:"user"`
}

// Login 用户登录并返回 JWT 令牌.
func (s *Service) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	user, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash()), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	now := time.Now()
	claims := servexJWT.StandardClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(user.ID(), 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
			Issuer:    "ecommerce-user-service",
		},
	}

	token, err := s.jwtSvc.Generate(ctx, claims)
	if err != nil {
		return nil, ErrGenerateToken.WithCause(err)
	}

	return &LoginResponse{
		Token: token,
		User:  domainUser.ToView(user),
	}, nil
}
