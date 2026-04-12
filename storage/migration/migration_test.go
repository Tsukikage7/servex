package migration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/Tsukikage7/servex/observability/logger"
)

// testDB 创建内存 SQLite 数据库.
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	require.NoError(t, err)
	return db
}

// testLogger 创建测试用日志记录器.
func testLogger(t *testing.T) logger.Logger {
	t.Helper()
	log, err := logger.NewLogger(&logger.Config{
		Type:   logger.TypeZap,
		Level:  logger.LevelDebug,
		Format: logger.FormatConsole,
		Output: logger.OutputConsole,
	})
	require.NoError(t, err)
	return log
}

// testRegistry 创建包含 3 个迁移的注册表.
func testRegistry() *Registry {
	return NewRegistry().
		Add(Migration{
			Version:     1,
			Description: "创建用户表",
			Up: func(tx *gorm.DB) error {
				return tx.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)").Error
			},
			Down: func(tx *gorm.DB) error {
				return tx.Exec("DROP TABLE users").Error
			},
		}).
		Add(Migration{
			Version:     3,
			Description: "创建订单表",
			Up: func(tx *gorm.DB) error {
				return tx.Exec("CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER)").Error
			},
			Down: func(tx *gorm.DB) error {
				return tx.Exec("DROP TABLE orders").Error
			},
		}).
		Add(Migration{
			Version:     2,
			Description: "创建产品表",
			Up: func(tx *gorm.DB) error {
				return tx.Exec("CREATE TABLE products (id INTEGER PRIMARY KEY, title TEXT)").Error
			},
			Down: func(tx *gorm.DB) error {
				return tx.Exec("DROP TABLE products").Error
			},
		})
}

func TestRegistry_Add_and_Sort(t *testing.T) {
	r := testRegistry()
	migrations := r.Migrations()

	require.Len(t, migrations, 3)
	assert.Equal(t, int64(1), migrations[0].Version)
	assert.Equal(t, int64(2), migrations[1].Version)
	assert.Equal(t, int64(3), migrations[2].Version)

	// 验证描述也正确.
	assert.Equal(t, "创建用户表", migrations[0].Description)
	assert.Equal(t, "创建产品表", migrations[1].Description)
	assert.Equal(t, "创建订单表", migrations[2].Description)
}

func TestRunner_Up(t *testing.T) {
	db := testDB(t)
	log := testLogger(t)
	registry := testRegistry()
	ctx := t.Context()

	rn, err := NewRunner(db, registry, log)
	require.NoError(t, err)

	// 执行所有迁移.
	err = rn.Up(ctx)
	require.NoError(t, err)

	// 验证所有迁移已应用.
	ver, err := rn.CurrentVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), ver)

	// 验证表已创建.
	var count int64
	require.NoError(t, db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&count).Error)
	assert.Equal(t, int64(1), count)
	require.NoError(t, db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='products'").Scan(&count).Error)
	assert.Equal(t, int64(1), count)
	require.NoError(t, db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='orders'").Scan(&count).Error)
	assert.Equal(t, int64(1), count)

	// 重复执行不应报错.
	err = rn.Up(ctx)
	require.NoError(t, err)
}

func TestRunner_Down(t *testing.T) {
	db := testDB(t)
	log := testLogger(t)
	registry := testRegistry()
	ctx := t.Context()

	rn, err := NewRunner(db, registry, log)
	require.NoError(t, err)

	// 先执行所有迁移.
	require.NoError(t, rn.Up(ctx))

	// 回滚最后一次.
	err = rn.Down(ctx)
	require.NoError(t, err)

	// 验证当前版本为 2.
	ver, err := rn.CurrentVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), ver)

	// 验证 orders 表已删除.
	var count int64
	require.NoError(t, db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='orders'").Scan(&count).Error)
	assert.Equal(t, int64(0), count)

	// users 和 products 表仍存在.
	require.NoError(t, db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestRunner_UpTo(t *testing.T) {
	db := testDB(t)
	log := testLogger(t)
	registry := testRegistry()
	ctx := t.Context()

	rn, err := NewRunner(db, registry, log)
	require.NoError(t, err)

	// 只迁移到版本 2.
	err = rn.UpTo(ctx, 2)
	require.NoError(t, err)

	ver, err := rn.CurrentVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), ver)

	// 验证 users 和 products 表存在，orders 不存在.
	var count int64
	require.NoError(t, db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&count).Error)
	assert.Equal(t, int64(1), count)
	require.NoError(t, db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='products'").Scan(&count).Error)
	assert.Equal(t, int64(1), count)
	require.NoError(t, db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='orders'").Scan(&count).Error)
	assert.Equal(t, int64(0), count)

	// 版本不存在时返回错误.
	err = rn.UpTo(ctx, 999)
	assert.ErrorIs(t, err, ErrVersionNotFound)
}

func TestRunner_Status(t *testing.T) {
	db := testDB(t)
	log := testLogger(t)
	registry := testRegistry()
	ctx := t.Context()

	rn, err := NewRunner(db, registry, log)
	require.NoError(t, err)

	// 执行到版本 2.
	require.NoError(t, rn.UpTo(ctx, 2))

	statuses, err := rn.Status(ctx)
	require.NoError(t, err)
	require.Len(t, statuses, 3)

	// 版本 1 和 2 已应用，版本 3 未应用.
	assert.True(t, statuses[0].Applied)
	assert.NotNil(t, statuses[0].AppliedAt)
	assert.True(t, statuses[1].Applied)
	assert.NotNil(t, statuses[1].AppliedAt)
	assert.False(t, statuses[2].Applied)
	assert.Nil(t, statuses[2].AppliedAt)
}

func TestRunner_CurrentVersion(t *testing.T) {
	db := testDB(t)
	log := testLogger(t)
	registry := testRegistry()
	ctx := t.Context()

	rn, err := NewRunner(db, registry, log)
	require.NoError(t, err)

	// 初始版本为 0.
	ver, err := rn.CurrentVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), ver)

	// 执行所有迁移后版本为 3.
	require.NoError(t, rn.Up(ctx))
	ver, err = rn.CurrentVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), ver)

	// 回滚一次后版本为 2.
	require.NoError(t, rn.Down(ctx))
	ver, err = rn.CurrentVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), ver)
}

func TestNewRunner_Validation(t *testing.T) {
	db := testDB(t)
	log := testLogger(t)
	registry := testRegistry()

	// db 为空.
	_, err := NewRunner(nil, registry, log)
	assert.ErrorIs(t, err, ErrNilDB)

	// registry 为空.
	_, err = NewRunner(db, nil, log)
	assert.ErrorIs(t, err, ErrNilRegistry)

	// logger 为空.
	_, err = NewRunner(db, registry, nil)
	assert.ErrorIs(t, err, ErrNilLogger)
}
