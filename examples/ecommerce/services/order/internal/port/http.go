// Package port 订单服务传输层.
package port

import (
	"context"
	"net/http"
	"strconv"

	"github.com/Tsukikage7/servex/v2/transport/httpserver"

	appOrder "github.com/Tsukikage7/servex/v2/examples/ecommerce/application/order"
	domainOrder "github.com/Tsukikage7/servex/v2/examples/ecommerce/domain/order"
)

// RegisterHTTPRoutes 注册订单服务的 HTTP 路由.
func RegisterHTTPRoutes(router *httpserver.Router, svc *appOrder.Service) {
	api := router.Group("/api/v1")

	api.POST("/orders", httpserver.Handle(placeOrderHandler(svc)))
	api.GET("/orders/{id}", httpserver.HandleWith(decodeOrderID, getOrderHandler(svc)))
	api.PUT("/orders/{id}/cancel", httpserver.HandleWith(decodeCancelOrder, cancelOrderHandler(svc)))
	api.PUT("/orders/{id}/ship", httpserver.HandleWith(decodeShipOrder, shipOrderHandler(svc)))
	api.PUT("/orders/{id}/complete", httpserver.HandleWith(decodeCompleteOrder, completeOrderHandler(svc)))
	api.GET("/orders", httpserver.HandleWith(decodeListOrders, listOrdersHandler(svc)))
}

// placeOrderHandler 下单.
func placeOrderHandler(svc *appOrder.Service) func(ctx context.Context, cmd domainOrder.PlaceOrderCommand) (*domainOrder.OrderView, error) {
	return func(ctx context.Context, cmd domainOrder.PlaceOrderCommand) (*domainOrder.OrderView, error) {
		return svc.PlaceOrder(ctx, cmd)
	}
}

// decodeOrderID 解析订单 ID.
func decodeOrderID(_ context.Context, r *http.Request) (domainOrder.GetOrderQuery, error) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		return domainOrder.GetOrderQuery{}, err
	}
	return domainOrder.GetOrderQuery{ID: id}, nil
}

// getOrderHandler 查询订单详情.
func getOrderHandler(svc *appOrder.Service) func(ctx context.Context, q domainOrder.GetOrderQuery) (*domainOrder.OrderView, error) {
	return func(ctx context.Context, q domainOrder.GetOrderQuery) (*domainOrder.OrderView, error) {
		return svc.GetByID(ctx, q.ID)
	}
}

// decodeCancelOrder 解析取消订单请求.
func decodeCancelOrder(_ context.Context, r *http.Request) (domainOrder.CancelOrderCommand, error) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		return domainOrder.CancelOrderCommand{}, err
	}
	return domainOrder.CancelOrderCommand{ID: id}, nil
}

// cancelOrderHandler 取消订单.
func cancelOrderHandler(svc *appOrder.Service) func(ctx context.Context, cmd domainOrder.CancelOrderCommand) (*domainOrder.OrderView, error) {
	return func(ctx context.Context, cmd domainOrder.CancelOrderCommand) (*domainOrder.OrderView, error) {
		return svc.CancelOrder(ctx, cmd)
	}
}

// decodeShipOrder 解析发货请求.
func decodeShipOrder(_ context.Context, r *http.Request) (domainOrder.ShipOrderCommand, error) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		return domainOrder.ShipOrderCommand{}, err
	}
	return domainOrder.ShipOrderCommand{ID: id}, nil
}

// shipOrderHandler 发货.
func shipOrderHandler(svc *appOrder.Service) func(ctx context.Context, cmd domainOrder.ShipOrderCommand) (*domainOrder.OrderView, error) {
	return func(ctx context.Context, cmd domainOrder.ShipOrderCommand) (*domainOrder.OrderView, error) {
		return svc.ShipOrder(ctx, cmd)
	}
}

// decodeCompleteOrder 解析完成订单请求.
func decodeCompleteOrder(_ context.Context, r *http.Request) (domainOrder.CompleteOrderCommand, error) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		return domainOrder.CompleteOrderCommand{}, err
	}
	return domainOrder.CompleteOrderCommand{ID: id}, nil
}

// completeOrderHandler 完成订单.
func completeOrderHandler(svc *appOrder.Service) func(ctx context.Context, cmd domainOrder.CompleteOrderCommand) (*domainOrder.OrderView, error) {
	return func(ctx context.Context, cmd domainOrder.CompleteOrderCommand) (*domainOrder.OrderView, error) {
		return svc.CompleteOrder(ctx, cmd)
	}
}

// decodeListOrders 解析查询订单列表的请求参数.
func decodeListOrders(_ context.Context, r *http.Request) (domainOrder.ListOrdersQuery, error) {
	var query domainOrder.ListOrdersQuery
	if uid := r.URL.Query().Get("user_id"); uid != "" {
		userID, err := strconv.ParseUint(uid, 10, 64)
		if err != nil {
			return query, err
		}
		query.UserID = userID
	}
	query.Offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	query.Limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if query.Limit <= 0 {
		query.Limit = 20
	}
	return query, nil
}

// listOrdersResponse 订单列表响应.
type listOrdersResponse struct {
	Orders []*domainOrder.OrderView `json:"orders"`
	Total  int64                    `json:"total"`
}

// listOrdersHandler 分页查询订单列表.
func listOrdersHandler(svc *appOrder.Service) func(ctx context.Context, q domainOrder.ListOrdersQuery) (*listOrdersResponse, error) {
	return func(ctx context.Context, q domainOrder.ListOrdersQuery) (*listOrdersResponse, error) {
		orders, total, err := svc.ListByUserID(ctx, q)
		if err != nil {
			return nil, err
		}
		return &listOrdersResponse{Orders: orders, Total: total}, nil
	}
}
