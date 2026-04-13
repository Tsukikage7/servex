// Package echoserver 提供基于 Echo 的类型安全 Handler 适配器.
//
// 在保留 Echo 原生路由、中间件体系的前提下，
// 统一请求解码、参数校验、响应包装与错误处理.
package echoserver

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/Tsukikage7/servex/v2/transport/response"
	"github.com/Tsukikage7/servex/v2/validation"
)

// Validatable 可由请求对象实现以启用自动校验.
//
// Handle/HandleWith 在解码后自动调用 Validate()，无需额外配置.
//
// Deprecated: 请直接使用 validation.Validatable.
type Validatable = validation.Validatable

// Handle 创建类型安全的 Echo HandlerFunc，自动处理 JSON 解码、校验与统一响应格式.
//
// 适用于请求体为 JSON 的场景（POST/PUT/PATCH）。
// 成功响应自动包装为 {"code":0,"message":"成功","data":{...}}；
// 若返回值已是 response.Response[T] 或 response.PagedResponse[T]，则不再二次包装。
// 错误自动转换为 {"code":xxxxx,"message":"..."} 并映射正确 HTTP 状态码.
//
// 示例：
//
//	e.POST("/users", echoserver.Handle(
//	    func(ctx context.Context, req CreateUserReq) (*UserResp, error) {
//	        return svc.CreateUser(ctx, req)
//	    },
//	))
func Handle[Req any, Resp any](fn func(ctx context.Context, req Req) (Resp, error)) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req Req
		if err := c.Bind(&req); err != nil {
			return writeError(c, response.NewError(response.CodeInvalidParam))
		}

		if v, ok := any(&req).(Validatable); ok {
			if err := v.Validate(); err != nil {
				return writeError(c, err)
			}
		}

		resp, err := fn(c.Request().Context(), req)
		if err != nil {
			return writeError(c, err)
		}

		return c.JSON(http.StatusOK, wrapEnvelope(resp))
	}
}

// HandleWith 创建带自定义解码器的类型安全 Echo HandlerFunc.
//
// 适用于需要从路径参数、查询字符串等位置提取请求数据的场景（GET/DELETE）.
// 响应包装规则与 Handle 相同.
//
// 示例：
//
//	e.GET("/users/:id", echoserver.HandleWith(
//	    func(c echo.Context) (GetUserReq, error) {
//	        return GetUserReq{ID: c.Param("id")}, nil
//	    },
//	    func(ctx context.Context, req GetUserReq) (*UserResp, error) {
//	        return svc.GetUser(ctx, req.ID)
//	    },
//	))
func HandleWith[Req any, Resp any](
	decode func(c echo.Context) (Req, error),
	fn func(ctx context.Context, req Req) (Resp, error),
) echo.HandlerFunc {
	return func(c echo.Context) error {
		req, err := decode(c)
		if err != nil {
			return writeError(c, err)
		}

		if v, ok := any(&req).(Validatable); ok {
			if err := v.Validate(); err != nil {
				return writeError(c, err)
			}
		}

		resp, err := fn(c.Request().Context(), req)
		if err != nil {
			return writeError(c, err)
		}

		return c.JSON(http.StatusOK, wrapEnvelope(resp))
	}
}

// writeError 写入统一格式的错误响应.
//
// 自动读取 Accept-Language 头并通过 i18n Bundle 翻译错误消息.
// 返回 nil 而非 error，因为响应已直接写入 ResponseWriter.
func writeError(c echo.Context, err error) error {
	code := response.ExtractCode(err)
	lang := c.Request().Header.Get("Accept-Language")
	message := response.LocalizedMessage(err, lang)

	return c.JSON(code.HTTPStatus, response.Response[any]{
		Code:    code.Num,
		Message: message,
	})
}

// wrapEnvelope 将响应包装在统一响应体中.
//
// 若 v 已实现 response.Envelope，直接返回，不做二次包装.
func wrapEnvelope(v any) any {
	if _, ok := v.(response.Envelope); ok {
		return v
	}
	return response.OK(v)
}
