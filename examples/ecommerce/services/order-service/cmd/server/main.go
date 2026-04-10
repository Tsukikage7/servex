// Package main 订单服务入口.
package main

import (
	"context"
	"log"

	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/app"
	"github.com/Tsukikage7/servex/domain"
	"github.com/Tsukikage7/servex/middleware/logging"
	"github.com/Tsukikage7/servex/middleware/recovery"
	"github.com/Tsukikage7/servex/observability/logger"
	"github.com/Tsukikage7/servex/storage/rdbms"
	"github.com/Tsukikage7/servex/transport/httpserver"

	appOrder "github.com/Tsukikage7/servex/examples/ecommerce/application/order"
	domainOrder "github.com/Tsukikage7/servex/examples/ecommerce/domain/order"
	"github.com/Tsukikage7/servex/examples/ecommerce/services/order-service/internal/adapter/external"
	"github.com/Tsukikage7/servex/examples/ecommerce/services/order-service/internal/adapter/persistence"
	"github.com/Tsukikage7/servex/examples/ecommerce/services/order-service/internal/port"
)

func main() {
	// 初始化日志
	l := logger.MustNewLogger(&logger.Config{
		Type:        logger.TypeZap,
		ServiceName: "order-service",
		Level:       logger.LevelInfo,
		Format:      logger.FormatConsole,
		Output:      logger.OutputConsole,
	})
	defer l.Close()

	// 初始化数据库
	db, err := rdbms.NewDatabase(&rdbms.Config{
		Type:   rdbms.TypeGORM,
		Driver: rdbms.DriverMySQL,
		DSN:    "root:password@tcp(127.0.0.1:3306)/ecommerce?charset=utf8mb4&parseTime=True&loc=Local",
	}, l)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer db.Close()

	// 自动迁移
	gormDB := db.DB().(*gorm.DB)
	if err := gormDB.AutoMigrate(&persistence.OrderPO{}); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	// 初始化事件总线
	eventBus := domain.NewEventBus()
	eventBus.Subscribe(domainOrder.EventOrderPlaced, func(ctx context.Context, event domain.DomainEvent) error {
		e := event.(*domainOrder.OrderPlacedEvent)
		l.With(
			logger.Uint64("order_id", e.OrderID),
			logger.Uint64("user_id", e.UserID),
		).Info("[Event] 订单创建成功")
		return nil
	})
	eventBus.Subscribe(domainOrder.EventOrderCancelled, func(ctx context.Context, event domain.DomainEvent) error {
		e := event.(*domainOrder.OrderCancelledEvent)
		l.With(logger.Uint64("order_id", e.OrderID)).Info("[Event] 订单已取消")
		return nil
	})
	eventBus.Subscribe(domainOrder.EventOrderShipped, func(ctx context.Context, event domain.DomainEvent) error {
		e := event.(*domainOrder.OrderShippedEvent)
		l.With(logger.Uint64("order_id", e.OrderID)).Info("[Event] 订单已发货")
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
		app.WithName("order-service"),
		app.WithVersion("1.0.0"),
		app.WithLogger(l),
	)
	if err := application.Use(httpSrv).Run(); err != nil {
		l.With(logger.Err(err)).Fatal("[App] 应用启动失败")
	}
}
