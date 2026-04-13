//go:build integration

package integration

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/v2/storage/rdbms"
	"github.com/Tsukikage7/servex/v2/testx"
)

// testUser GORM test model.
type testUser struct {
	rdbms.BaseModel[uint]
	Name  string `gorm:"type:varchar(100);not null"`
	Email string `gorm:"type:varchar(255);uniqueIndex"`
	Age   int
}

func (testUser) TableName() string {
	return "inttest_users"
}

func postgresDSN() string {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = "host=localhost port=5432 user=test password=test dbname=servex_test sslmode=disable"
	}
	return dsn
}

func newPostgresDB(t *testing.T) rdbms.Database {
	t.Helper()

	cfg := rdbms.DefaultConfig()
	cfg.Driver = rdbms.DriverPostgres
	cfg.DSN = postgresDSN()
	cfg.AutoMigrate = true
	cfg.LogLevel = "silent"

	db, err := rdbms.NewDatabase(cfg, testx.NopLogger())
	if err != nil {
		t.Skipf("Postgres not available: %v", err)
		return nil
	}

	t.Cleanup(func() {
		// Drop test table
		gormDB := rdbms.AsGORM(db)
		gormDB.Exec("DROP TABLE IF EXISTS inttest_users")
		db.Close()
	})

	return db
}

func TestRDBMS_Integration(t *testing.T) {
	database := newPostgresDB(t)
	ctx := t.Context()

	// AutoMigrate
	err := database.AutoMigrate(&testUser{})
	require.NoError(t, err)

	gormDB := rdbms.AsGORM(database)

	// Clean table before tests
	gormDB.WithContext(ctx).Exec("TRUNCATE TABLE inttest_users RESTART IDENTITY CASCADE")

	t.Run("Create_Read", func(t *testing.T) {
		user := testUser{
			Name:  "alice",
			Email: "alice@example.com",
			Age:   30,
		}
		result := gormDB.WithContext(ctx).Create(&user)
		require.NoError(t, result.Error)
		assert.NotZero(t, user.ID)
		assert.False(t, user.CreatedAt.IsZero())
		assert.False(t, user.UpdatedAt.IsZero())

		// Read
		var found testUser
		result = gormDB.WithContext(ctx).First(&found, user.ID)
		require.NoError(t, result.Error)
		assert.Equal(t, "alice", found.Name)
		assert.Equal(t, "alice@example.com", found.Email)
		assert.Equal(t, 30, found.Age)
	})

	t.Run("Update", func(t *testing.T) {
		user := testUser{Name: "bob", Email: "bob@example.com", Age: 25}
		gormDB.WithContext(ctx).Create(&user)

		result := gormDB.WithContext(ctx).Model(&user).Update("age", 26)
		require.NoError(t, result.Error)
		assert.Equal(t, int64(1), result.RowsAffected)

		var found testUser
		gormDB.WithContext(ctx).First(&found, user.ID)
		assert.Equal(t, 26, found.Age)
	})

	t.Run("SoftDelete", func(t *testing.T) {
		user := testUser{Name: "charlie", Email: "charlie@example.com", Age: 35}
		gormDB.WithContext(ctx).Create(&user)

		// Soft delete
		result := gormDB.WithContext(ctx).Delete(&user)
		require.NoError(t, result.Error)

		// Should not find with normal query
		var notFound testUser
		result = gormDB.WithContext(ctx).First(&notFound, user.ID)
		assert.ErrorIs(t, result.Error, gorm.ErrRecordNotFound)

		// Should find with Unscoped
		var found testUser
		result = gormDB.WithContext(ctx).Unscoped().First(&found, user.ID)
		require.NoError(t, result.Error)
		assert.True(t, found.DeletedAt.Valid)
	})

	t.Run("Query", func(t *testing.T) {
		// Clean and insert fresh data
		gormDB.WithContext(ctx).Exec("TRUNCATE TABLE inttest_users RESTART IDENTITY CASCADE")

		users := []testUser{
			{Name: "user1", Email: "u1@example.com", Age: 20},
			{Name: "user2", Email: "u2@example.com", Age: 25},
			{Name: "user3", Email: "u3@example.com", Age: 30},
		}
		gormDB.WithContext(ctx).Create(&users)

		// Where
		var results []testUser
		gormDB.WithContext(ctx).Where("age >= ?", 25).Find(&results)
		assert.Len(t, results, 2)

		// Order + Limit
		var ordered []testUser
		gormDB.WithContext(ctx).Order("age DESC").Limit(2).Find(&ordered)
		assert.Len(t, ordered, 2)
		assert.Equal(t, 30, ordered[0].Age)

		// Count
		var count int64
		gormDB.WithContext(ctx).Model(&testUser{}).Count(&count)
		assert.Equal(t, int64(3), count)
	})

	t.Run("Transaction", func(t *testing.T) {
		gormDB.WithContext(ctx).Exec("TRUNCATE TABLE inttest_users RESTART IDENTITY CASCADE")

		// Successful transaction
		err := gormDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			tx.Create(&testUser{Name: "tx1", Email: "tx1@example.com", Age: 1})
			tx.Create(&testUser{Name: "tx2", Email: "tx2@example.com", Age: 2})
			return nil
		})
		require.NoError(t, err)

		var count int64
		gormDB.WithContext(ctx).Model(&testUser{}).Count(&count)
		assert.Equal(t, int64(2), count)

		// Rolled-back transaction
		err = gormDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			tx.Create(&testUser{Name: "tx3", Email: "tx3@example.com", Age: 3})
			return gorm.ErrInvalidTransaction // force rollback
		})
		assert.Error(t, err)

		gormDB.WithContext(ctx).Model(&testUser{}).Count(&count)
		assert.Equal(t, int64(2), count) // still 2
	})

	t.Run("DB_helper", func(t *testing.T) {
		// Test the rdbms.DB() helper
		db := rdbms.DB(ctx, database)
		assert.NotNil(t, db)

		var count int64
		result := db.Model(&testUser{}).Count(&count)
		require.NoError(t, result.Error)
	})
}

func TestRDBMS_BaseModel(t *testing.T) {
	database := newPostgresDB(t)
	ctx := t.Context()

	err := database.AutoMigrate(&testUser{})
	require.NoError(t, err)

	gormDB := rdbms.AsGORM(database)
	gormDB.WithContext(ctx).Exec("TRUNCATE TABLE inttest_users RESTART IDENTITY CASCADE")

	user := testUser{Name: "timestamp_test", Email: "ts@example.com", Age: 1}
	gormDB.WithContext(ctx).Create(&user)

	assert.NotZero(t, user.CreatedAt)
	assert.NotZero(t, user.UpdatedAt)

	originalUpdated := user.UpdatedAt
	time.Sleep(10 * time.Millisecond)

	gormDB.WithContext(ctx).Model(&user).Update("age", 2)

	var reloaded testUser
	gormDB.WithContext(ctx).First(&reloaded, user.ID)
	assert.True(t, reloaded.UpdatedAt.After(originalUpdated) || reloaded.UpdatedAt.Equal(originalUpdated))
}
