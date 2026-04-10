package health

import (
	"context"
	"database/sql"
)

// Pinger 实现了 Ping 方法的接口.
//
// 常见实现: *sql.DB, *redis.Client, *gorm.DB 等.
type Pinger interface {
	Ping(ctx context.Context) error
}

// SQLPinger 将 *sql.DB 适配为 Pinger 接口.
type SQLPinger struct {
	db *sql.DB
}

// NewSQLPinger 创建 SQL Pinger.
func NewSQLPinger(db *sql.DB) *SQLPinger {
	return &SQLPinger{db: db}
}

// Ping 执行数据库 Ping.
func (p *SQLPinger) Ping(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

// PingChecker 通用 Ping 检查器.
//
// 可用于任何实现了 Pinger 接口的组件.
type PingChecker struct {
	name   string
	pinger Pinger
}

// NewPingChecker 创建 Ping 检查器.
func NewPingChecker(name string, pinger Pinger) *PingChecker {
	return &PingChecker{
		name:   name,
		pinger: pinger,
	}
}

// Name 返回检查器名称.
func (c *PingChecker) Name() string {
	return c.name
}

// Check 执行 Ping 检查.
func (c *PingChecker) Check(ctx context.Context) CheckResult {
	if err := c.pinger.Ping(ctx); err != nil {
		return CheckResult{
			Status:  StatusDown,
			Message: err.Error(),
		}
	}
	return CheckResult{
		Status: StatusUp,
	}
}

// DBChecker 数据库健康检查器.
//
// 支持 *sql.DB 和任何实现了 Pinger 接口的数据库连接.
type DBChecker struct {
	name   string
	pinger Pinger
}

// NewDBChecker 创建数据库检查器.
//
// db 可以是 *sql.DB 或任何实现了 Pinger 接口的对象.
// 对于 GORM，可以通过 db.DB() 获取 *sql.DB.
func NewDBChecker(name string, pinger Pinger) *DBChecker {
	return &DBChecker{
		name:   name,
		pinger: pinger,
	}
}

// NewDBCheckerFromSQL 从 *sql.DB 创建数据库检查器.
func NewDBCheckerFromSQL(name string, db *sql.DB) *DBChecker {
	return &DBChecker{
		name:   name,
		pinger: NewSQLPinger(db),
	}
}

// Name 返回检查器名称.
func (c *DBChecker) Name() string {
	return c.name
}

// Check 执行数据库健康检查.
func (c *DBChecker) Check(ctx context.Context) CheckResult {
	if err := c.pinger.Ping(ctx); err != nil {
		return CheckResult{
			Status:  StatusDown,
			Message: err.Error(),
			Details: map[string]any{
				"type": "database",
			},
		}
	}
	return CheckResult{
		Status: StatusUp,
		Details: map[string]any{
			"type": "database",
		},
	}
}

// RedisChecker Redis 健康检查器.
type RedisChecker struct {
	name   string
	pinger Pinger
}

// NewRedisChecker 创建 Redis 检查器.
//
// pinger 需要实现 Ping(ctx context.Context) error 方法.
// go-redis/redis 的 *redis.Client 已实现此接口.
func NewRedisChecker(name string, pinger Pinger) *RedisChecker {
	return &RedisChecker{
		name:   name,
		pinger: pinger,
	}
}

// Name 返回检查器名称.
func (c *RedisChecker) Name() string {
	return c.name
}

// Check 执行 Redis 健康检查.
func (c *RedisChecker) Check(ctx context.Context) CheckResult {
	if err := c.pinger.Ping(ctx); err != nil {
		return CheckResult{
			Status:  StatusDown,
			Message: err.Error(),
			Details: map[string]any{
				"type": "redis",
			},
		}
	}
	return CheckResult{
		Status: StatusUp,
		Details: map[string]any{
			"type": "redis",
		},
	}
}

// CompositeChecker 组合检查器，将多个检查器组合为一个.
type CompositeChecker struct {
	name     string
	checkers []Checker
}

// NewCompositeChecker 创建组合检查器.
func NewCompositeChecker(name string, checkers ...Checker) *CompositeChecker {
	return &CompositeChecker{
		name:     name,
		checkers: checkers,
	}
}

// Name 返回检查器名称.
func (c *CompositeChecker) Name() string {
	return c.name
}

// Check 执行所有子检查器，任一失败则整体失败.
// 当前串行执行各子检查器；如需并行可由调用方使用独立的 goroutine.
func (c *CompositeChecker) Check(ctx context.Context) CheckResult {
	details := make(map[string]any)
	overallStatus := StatusUp

	for _, checker := range c.checkers {
		result := checker.Check(ctx)
		details[checker.Name()] = result

		if result.Status == StatusDown {
			overallStatus = StatusDown
		} else if result.Status == StatusUnknown && overallStatus != StatusDown {
			overallStatus = StatusUnknown
		}
	}

	return CheckResult{
		Status:  overallStatus,
		Details: details,
	}
}

// AlwaysUpChecker 始终返回 UP 的检查器，用于基本存活检查.
type AlwaysUpChecker struct {
	name string
}

// NewAlwaysUpChecker 创建始终健康的检查器.
func NewAlwaysUpChecker(name string) *AlwaysUpChecker {
	return &AlwaysUpChecker{name: name}
}

// Name 返回检查器名称.
func (c *AlwaysUpChecker) Name() string {
	return c.name
}

// Check 始终返回 UP.
func (c *AlwaysUpChecker) Check(_ context.Context) CheckResult {
	return CheckResult{Status: StatusUp}
}
