// Package port 用户服务传输层.
package port

import (
	"context"
	"net/http"
	"strconv"

	"github.com/Tsukikage7/servex/transport/httpserver"

	appUser "github.com/Tsukikage7/servex/examples/ecommerce/application/user"
	domainUser "github.com/Tsukikage7/servex/examples/ecommerce/domain/user"
)

// RegisterHTTPRoutes 注册用户服务的 HTTP 路由.
func RegisterHTTPRoutes(router *httpserver.Router, svc *appUser.Service) {
	api := router.Group("/api/v1")

	api.POST("/users", httpserver.Handle(createUserHandler(svc)))
	api.GET("/users/{id}", httpserver.HandleWith(decodeGetUser, getUserHandler(svc)))
	api.PUT("/users/{id}", httpserver.HandleWith(decodeUpdateUser, updateUserHandler(svc)))
	api.GET("/users", httpserver.HandleWith(decodeListUsers, listUsersHandler(svc)))
	api.POST("/auth/login", httpserver.Handle(loginHandler(svc)))
}

// createUserHandler 创建用户.
func createUserHandler(svc *appUser.Service) func(ctx context.Context, cmd domainUser.CreateUserCommand) (*domainUser.UserView, error) {
	return func(ctx context.Context, cmd domainUser.CreateUserCommand) (*domainUser.UserView, error) {
		return svc.Create(ctx, cmd)
	}
}

// decodeGetUser 解析查询单个用户的请求参数.
func decodeGetUser(_ context.Context, r *http.Request) (domainUser.GetUserQuery, error) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		return domainUser.GetUserQuery{}, err
	}
	return domainUser.GetUserQuery{ID: id}, nil
}

// getUserHandler 查询单个用户.
func getUserHandler(svc *appUser.Service) func(ctx context.Context, q domainUser.GetUserQuery) (*domainUser.UserView, error) {
	return func(ctx context.Context, q domainUser.GetUserQuery) (*domainUser.UserView, error) {
		return svc.GetByID(ctx, q.ID)
	}
}

// decodeUpdateUser 解析更新用户的请求参数.
func decodeUpdateUser(ctx context.Context, r *http.Request) (domainUser.UpdateUserCommand, error) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		return domainUser.UpdateUserCommand{}, err
	}
	// 仅解析 URL 中的 ID，Body 部分由框架自动解码
	return domainUser.UpdateUserCommand{ID: id}, nil
}

// updateUserHandler 更新用户.
func updateUserHandler(svc *appUser.Service) func(ctx context.Context, cmd domainUser.UpdateUserCommand) (*domainUser.UserView, error) {
	return func(ctx context.Context, cmd domainUser.UpdateUserCommand) (*domainUser.UserView, error) {
		return svc.Update(ctx, cmd)
	}
}

// decodeListUsers 解析分页查询用户列表的请求参数.
func decodeListUsers(_ context.Context, r *http.Request) (domainUser.ListUsersQuery, error) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	return domainUser.ListUsersQuery{Offset: offset, Limit: limit}, nil
}

// listUsersResponse 用户列表响应.
type listUsersResponse struct {
	Users []*domainUser.UserView `json:"users"`
	Total int64                  `json:"total"`
}

// listUsersHandler 分页查询用户列表.
func listUsersHandler(svc *appUser.Service) func(ctx context.Context, q domainUser.ListUsersQuery) (*listUsersResponse, error) {
	return func(ctx context.Context, q domainUser.ListUsersQuery) (*listUsersResponse, error) {
		users, total, err := svc.List(ctx, q)
		if err != nil {
			return nil, err
		}
		return &listUsersResponse{Users: users, Total: total}, nil
	}
}

// loginHandler 用户登录.
func loginHandler(svc *appUser.Service) func(ctx context.Context, req appUser.LoginRequest) (*appUser.LoginResponse, error) {
	return func(ctx context.Context, req appUser.LoginRequest) (*appUser.LoginResponse, error) {
		return svc.Login(ctx, req)
	}
}
