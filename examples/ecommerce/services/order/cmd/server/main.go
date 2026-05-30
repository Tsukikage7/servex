// Package main 订单服务入口.
package main

import (
	"context"

	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/v2/app"
	"github.com/Tsukikage7/servex/v2/domain"
	"github.com/Tsukikage7/servex/v2/middleware/logging"
	"github.com/Tsukikage7/servex/v2/middleware/recovery"
	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/storage/rdbms"
	"github.com/Tsukikage7/servex/v2/transport/httpserver"

	appOrder "github.com/Tsukikage7/servex/v2/examples/ecommerce/application/order"
	domainOrder "github.com/Tsukikage7/servex/v2/examples/ecommerce/domain/order"
	"github.com/Tsukikage7/servex/v2/examples/ecommerce/services/order/internal/adapter/external"
	"github.com/Tsukikage7/servex/v2/examples/ecommerce/services/order/internal/adapter/persistence"
	"github.com/Tsukikage7/servex/v2/examples/ecommerce/services/order/internal/port"
)

func main() {
	// 初始化日志
	l := logger.MustNewLogger(&logger.Config{
		Type:        logger.TypeZap,
		ServiceName: "order",
		Level:       logger.LevelInfo,
		Format:      logger.FormatConsole,
		Output:      logger.OutputConsole,
	})
	defer l.Close()
	mainLog := logger.WithComponent(l, "Ecommerce")
	eventLog := logger.WithComponent(l, "Event")

	// 初始化数据库
	db, err := rdbms.NewDatabase(&rdbms.Config{
		Type:   rdbms.TypeGORM,
		Driver: rdbms.DriverMySQL,
		DSN:    "root:password@tcp(127.0.0.1:3306)/ecommerce?charset=utf8mb4&parseTime=True&loc=Local",
	}, l)
	if err != nil {
		mainLog.With(logger.Err(err)).Fatal("初始化数据库失败")
	}
	defer db.Close()

	// 自动迁移
	gormDB := db.DB().(*gorm.DB)
	if err := gormDB.AutoMigrate(&persistence.OrderPO{}); err != nil {
		mainLog.With(logger.Err(err)).Fatal("数据库迁移失败")
	}

	// 初始化事件总线
	eventBus := domain.NewEventBus()
	eventBus.Subscribe(domainOrder.EventOrderPlaced, func(ctx context.Context, event domain.DomainEvent) error {
		e := event.(*domainOrder.OrderPlacedEvent)
		eventLog.With(
			logger.Uint64("order_id", e.OrderID),
			logger.Uint64("user_id", e.UserID),
		).Info("订单创建成功")
		return nil
	})
	eventBus.Subscribe(domainOrder.EventOrderCancelled, func(ctx context.Context, event domain.DomainEvent) error {
		e := event.(*domainOrder.OrderCancelledEvent)
		eventLog.With(logger.Uint64("order_id", e.OrderID)).Info("订单已取消")
		return nil
	})
	eventBus.Subscribe(domainOrder.EventOrderShipped, func(ctx context.Context, event domain.DomainEvent) error {
		e := event.(*domainOrder.OrderShippedEvent)
		eventLog.With(logger.Uint64("order_id", e.OrderID)).Info("订单已发货")
		return nil
	})

	// 初始化外部服务客户端
	userClient := external.NewUserClient("http://127.0.0.1:8081")

	// 初始化仓储与应用服务
	orderRepo := persistence.NewOrderRepository(gormDB)
	orderSvc := appOrder.NewService(orderRepo, eventBus, userClient)

	// 初始化 HTTP 路由
	router := httpserver.NewRouter()
	port.RegisterHTTPRoutes(router, orderSvc)

	// 创建 HTTP 服务器
	httpSrv := httpserver.New(router,
		httpserver.WithLogger(l),
		httpserver.WithAddr(":8082"),
		httpserver.WithMiddlewares(
			recovery.HTTPMiddleware(recovery.WithLogger(l)),
			logging.HTTPMiddleware(logging.WithLogger(l)),
		),
	)

	// 启动应用
	application := app.New(
		app.WithName("order"),
		app.WithVersion("1.0.0"),
		app.WithLogger(l),
	)
	if err := application.Use(httpSrv).Run(); err != nil {
		l.With(logger.Err(err)).Fatal("应用启动失败")
	}
}
