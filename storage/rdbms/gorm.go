package rdbms

import (
	"context"
	"errors"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/plugin/opentelemetry/tracing"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// BaseModel GORM 基础模型.
type BaseModel[T any] struct {
	ID        T              `gorm:"primaryKey"`
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

// gormDatabase GORM 数据库实现.
type gormDatabase struct {
	db     *gorm.DB
	config *Config
	logger logger.Logger
}

// newGORMDatabase 创建 GORM 数据库连接.
func newGORMDatabase(config *Config, log logger.Logger) (Database, error) {
	log = logger.WithComponent(log, "Database")

	dialector, err := getDialector(config.Driver, config.DSN)
	if err != nil {
		return nil, err
	}

	gormConfig := &gorm.Config{
		Logger: newGORMLoggerAdapter(log, config.SlowThreshold, config.LogLevel),
	}

	db, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		return nil, err
	}

	// 注册链路追踪插件
	if config.EnableTracing {
		if err = db.Use(tracing.NewPlugin()); err != nil {
			return nil, errors.Join(ErrRegisterTracingPlugin, err)
		}
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(config.Pool.MaxOpen)
	sqlDB.SetMaxIdleConns(config.Pool.MaxIdle)
	sqlDB.SetConnMaxLifetime(config.Pool.MaxLifetime)
	sqlDB.SetConnMaxIdleTime(config.Pool.MaxIdleTime)

	return &gormDatabase{
		db:     db,
		config: config,
		logger: log,
	}, nil
}

// getDialector 根据驱动类型返回对应的 Dialector.
func getDialector(driver, dsn string) (gorm.Dialector, error) {
	switch driver {
	case DriverMySQL:
		return mysql.Open(dsn), nil
	case DriverPostgres, DriverPostgreSQL:
		return postgres.Open(dsn), nil
	case DriverSQLite, DriverSQLite3:
		return sqlite.Open(dsn), nil
	default:
		return nil, ErrUnsupportedDriver
	}
}

// DB 获取 GORM 数据库实例.
func (g *gormDatabase) DB() any {
	return g.db
}

// GORM 获取类型安全的 GORM 实例.
func (g *gormDatabase) GORM() *gorm.DB {
	return g.db
}

// AutoMigrate 自动迁移表结构.
func (g *gormDatabase) AutoMigrate(models ...any) error {
	if !g.config.AutoMigrate {
		g.logger.Debug("自动迁移已禁用，跳过表结构创建")
		return nil
	}

	g.logger.Debug("开始自动迁移表结构")
	if err := g.db.AutoMigrate(models...); err != nil {
		g.logger.Error("自动迁移失败", logger.Err(err))
		return err
	}
	g.logger.Debug("表结构迁移完成")
	return nil
}

// Close 关闭数据库连接.
func (g *gormDatabase) Close() error {
	sqlDB, err := g.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// GORMDatabase GORM 数据库扩展接口.
type GORMDatabase interface {
	GORM() *gorm.DB
}

// AsGORM 将 Database 转换为 *gorm.DB.
//
// 注意: 如果启用了链路追踪，请使用 DB(ctx) 方法以确保追踪生效.
func AsGORM(db Database) *gorm.DB {
	if gdb, ok := db.(*gormDatabase); ok {
		return gdb.db
	}
	panic("database: 无法提取 *gorm.DB，请确保使用 GORM 类型的数据库")
}

// DB 获取带 context 的 *gorm.DB（推荐）.
//
// 使用此方法可确保链路追踪正常工作:
//
//	database.DB(ctx, db).Find(&users)
func DB(ctx context.Context, db Database) *gorm.DB {
	return AsGORM(db).WithContext(ctx)
}

// gormLoggerAdapter GORM 日志适配器.
type gormLoggerAdapter struct {
	logger        logger.Logger
	slowThreshold time.Duration
	logLevel      gormlogger.LogLevel
}

// newGORMLoggerAdapter 创建 GORM 日志适配器.
func newGORMLoggerAdapter(log logger.Logger, slowThreshold time.Duration, level string) gormlogger.Interface {
	logLevel := gormlogger.Info
	switch level {
	case "silent":
		logLevel = gormlogger.Silent
	case "error":
		logLevel = gormlogger.Error
	case "warn":
		logLevel = gormlogger.Warn
	case "info":
		logLevel = gormlogger.Info
	}

	return &gormLoggerAdapter{
		logger:        log,
		slowThreshold: slowThreshold,
		logLevel:      logLevel,
	}
}

// LogMode 设置日志模式.
func (l *gormLoggerAdapter) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.logLevel = level
	return &newLogger
}

func (l *gormLoggerAdapter) log(ctx context.Context) logger.Logger {
	return logger.For(ctx, "Database")
}

// Info 信息日志.
func (l *gormLoggerAdapter) Info(ctx context.Context, msg string, data ...any) {
	if l.logLevel >= gormlogger.Info {
		l.log(ctx).Infof(msg, data...)
	}
}

// Warn 警告日志.
func (l *gormLoggerAdapter) Warn(ctx context.Context, msg string, data ...any) {
	if l.logLevel >= gormlogger.Warn {
		l.log(ctx).Warnf(msg, data...)
	}
}

// Error 错误日志.
func (l *gormLoggerAdapter) Error(ctx context.Context, msg string, data ...any) {
	if l.logLevel >= gormlogger.Error {
		l.log(ctx).Errorf(msg, data...)
	}
}

// Trace SQL 跟踪日志.
func (l *gormLoggerAdapter) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.logLevel <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	// 使用结构化字段，让 logger 根据 Format 配置自动格式化
	// 先添加业务字段，再添加 trace 信息，保持 traceId/spanId 在最后
	log := l.log(ctx).With(
		logger.Duration("elapsed", elapsed),
		logger.Int64("rows", rows),
		logger.String("sql", sql),
	)

	switch {
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
		log.With(logger.Err(err)).Error("SQL 执行失败")
	case elapsed > l.slowThreshold && l.slowThreshold > 0:
		log.With(logger.Duration("threshold", l.slowThreshold)).Warn("慢查询")
	default:
		// 根据配置级别输出：info 级别显示 SQL，低于 info 则不显示
		if l.logLevel >= gormlogger.Info {
			log.Info("SQL 执行成功")
		}
	}
}
