// Package main 用户服务入口.
package main

import (
	"context"

	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/v2/app"
	"github.com/Tsukikage7/servex/v2/auth/jwt"
	"github.com/Tsukikage7/servex/v2/domain"
	"github.com/Tsukikage7/servex/v2/middleware/logging"
	"github.com/Tsukikage7/servex/v2/middleware/recovery"
	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/storage/rdbms"
	"github.com/Tsukikage7/servex/v2/transport/httpserver"

	appUser "github.com/Tsukikage7/servex/v2/examples/ecommerce/application/user"
	domainUser "github.com/Tsukikage7/servex/v2/examples/ecommerce/domain/user"
	"github.com/Tsukikage7/servex/v2/examples/ecommerce/services/user/internal/adapter/persistence"
	"github.com/Tsukikage7/servex/v2/examples/ecommerce/services/user/internal/port"
)

func main() {
	// 初始化日志
	l := logger.MustNewLogger(&logger.Config{
		Type:        logger.TypeZap,
		ServiceName: "user",
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
	if err := gormDB.AutoMigrate(&persistence.UserPO{}); err != nil {
		mainLog.With(logger.Err(err)).Fatal("数据库迁移失败")
	}

	// 初始化 JWT 服务
	jwtSvc := jwt.MustNew(
		jwt.WithSecretKey("ecommerce-secret-key-please-change-in-production"),
		jwt.WithIssuer("ecommerce-user"),
		jwt.WithLogger(l),
	)

	// 初始化事件总线
	eventBus := domain.NewEventBus()
	eventBus.Subscribe(domainUser.EventUserCreated, func(ctx context.Context, event domain.DomainEvent) error {
		e := event.(*domainUser.UserCreatedEvent)
		eventLog.With(
			logger.Uint64("user_id", e.UserID),
			logger.String("username", e.Username),
		).Info("用户创建成功")
		return nil
	})

	// 初始化仓储与应用服务
	userRepo := persistence.NewUserRepository(gormDB)
	userSvc := appUser.NewService(userRepo, eventBus, jwtSvc)

	// 初始化 HTTP 路由
	router := httpserver.NewRouter()
	port.RegisterHTTPRoutes(router, userSvc)

	// 创建 HTTP 服务器
	httpSrv := httpserver.New(router,
		httpserver.WithLogger(l),
		httpserver.WithAddr(":8081"),
		httpserver.WithMiddlewares(
			recovery.HTTPMiddleware(recovery.WithLogger(l)),
			logging.HTTPMiddleware(logging.WithLogger(l)),
		),
	)

	// 启动应用
	application := app.New(
		app.WithName("user"),
		app.WithVersion("1.0.0"),
		app.WithLogger(l),
	)
	if err := application.Use(httpSrv).Run(); err != nil {
		l.With(logger.Err(err)).Fatal("应用启动失败")
	}
}
