//go:build integration

package integration

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/Tsukikage7/servex/storage/migration"
	"github.com/Tsukikage7/servex/testx"
)

func newMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = "host=localhost port=5432 user=test password=test dbname=servex_test sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Skipf("Postgres not available for migration test: %v", err)
		return nil
	}

	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS schema_migrations")
		db.Exec("DROP TABLE IF EXISTS inttest_migration_items")
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})

	// Clean up before test
	db.Exec("DROP TABLE IF EXISTS schema_migrations")
	db.Exec("DROP TABLE IF EXISTS inttest_migration_items")

	return db
}

func testRegistry() *migration.Registry {
	return migration.NewRegistry().
		Add(migration.Migration{
			Version:     20240101000001,
			Description: "create items table",
			Up: func(tx *gorm.DB) error {
				return tx.Exec(`CREATE TABLE inttest_migration_items (
					id SERIAL PRIMARY KEY,
					name VARCHAR(100) NOT NULL,
					created_at TIMESTAMP DEFAULT NOW()
				)`).Error
			},
			Down: func(tx *gorm.DB) error {
				return tx.Exec("DROP TABLE IF EXISTS inttest_migration_items").Error
			},
		}).
		Add(migration.Migration{
			Version:     20240101000002,
			Description: "add price column",
			Up: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE inttest_migration_items ADD COLUMN price NUMERIC(10,2) DEFAULT 0").Error
			},
			Down: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE inttest_migration_items DROP COLUMN IF EXISTS price").Error
			},
		}).
		Add(migration.Migration{
			Version:     20240101000003,
			Description: "add status column",
			Up: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE inttest_migration_items ADD COLUMN status VARCHAR(20) DEFAULT 'active'").Error
			},
			Down: func(tx *gorm.DB) error {
				return tx.Exec("ALTER TABLE inttest_migration_items DROP COLUMN IF EXISTS status").Error
			},
		})
}

func TestMigration_Integration(t *testing.T) {
	db := newMigrationDB(t)
	ctx := t.Context()

	t.Run("Up_All", func(t *testing.T) {
		registry := testRegistry()
		runner, err := migration.NewRunner(db, registry, testx.NopLogger())
		require.NoError(t, err)

		// Run all migrations up
		err = runner.Up(ctx)
		require.NoError(t, err)

		// Check current version
		version, err := runner.CurrentVersion(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(20240101000003), version)

		// Verify table exists with all columns
		result := db.Exec("INSERT INTO inttest_migration_items (name, price, status) VALUES ('test', 9.99, 'active')")
		require.NoError(t, result.Error)

		// Running Up again should be idempotent
		err = runner.Up(ctx)
		require.NoError(t, err)
	})

	t.Run("Status", func(t *testing.T) {
		registry := testRegistry()
		runner, err := migration.NewRunner(db, registry, testx.NopLogger())
		require.NoError(t, err)

		statuses, err := runner.Status(ctx)
		require.NoError(t, err)
		assert.Len(t, statuses, 3)

		for _, s := range statuses {
			assert.True(t, s.Applied, "version %d should be applied", s.Version)
			assert.NotNil(t, s.AppliedAt)
		}
	})

	t.Run("Down", func(t *testing.T) {
		registry := testRegistry()
		runner, err := migration.NewRunner(db, registry, testx.NopLogger())
		require.NoError(t, err)

		// Down last migration
		err = runner.Down(ctx)
		require.NoError(t, err)

		version, err := runner.CurrentVersion(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(20240101000002), version)

		// Verify status column is gone
		result := db.Exec("SELECT status FROM inttest_migration_items LIMIT 1")
		assert.Error(t, result.Error) // column should not exist
	})

	t.Run("DownTo", func(t *testing.T) {
		registry := testRegistry()
		runner, err := migration.NewRunner(db, registry, testx.NopLogger())
		require.NoError(t, err)

		// Rollback to version 1 (keep version 1, remove version 2)
		err = runner.DownTo(ctx, 20240101000001)
		require.NoError(t, err)

		version, err := runner.CurrentVersion(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(20240101000001), version)
	})

	t.Run("UpTo", func(t *testing.T) {
		registry := testRegistry()
		runner, err := migration.NewRunner(db, registry, testx.NopLogger())
		require.NoError(t, err)

		// Up to version 2 only
		err = runner.UpTo(ctx, 20240101000002)
		require.NoError(t, err)

		version, err := runner.CurrentVersion(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(20240101000002), version)

		// Status should show version 3 as not applied
		statuses, err := runner.Status(ctx)
		require.NoError(t, err)
		for _, s := range statuses {
			if s.Version == 20240101000003 {
				assert.False(t, s.Applied)
			} else {
				assert.True(t, s.Applied)
			}
		}
	})
}
