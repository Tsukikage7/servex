package rdbms

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// DatabaseTestSuite 数据库测试套件.
type DatabaseTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func TestDatabaseSuite(t *testing.T) {
	suite.Run(t, new(DatabaseTestSuite))
}

func (s *DatabaseTestSuite) SetupSuite() {
	log, err := logger.NewLogger(logger.DefaultConfig())
	s.Require().NoError(err)
	s.logger = log
}

func (s *DatabaseTestSuite) TearDownSuite() {
	if s.logger != nil {
		s.logger.Close()
	}
}

func (s *DatabaseTestSuite) TestErrors() {
	s.Equal("database: 配置为空", ErrNilConfig.Error())
	s.Equal("database: 日志记录器为空", ErrNilLogger.Error())
	s.Equal("database: 驱动类型为空", ErrEmptyDriver.Error())
	s.Equal("database: 连接字符串为空", ErrEmptyDSN.Error())
	s.Equal("database: 不支持的驱动类型", ErrUnsupportedDriver.Error())
	s.Equal("database: 不支持的 ORM 类型", ErrUnsupportedType.Error())
	s.Equal("database: 注册追踪插件失败", ErrRegisterTracingPlugin.Error())
	s.Equal("database: 数据库为空", ErrNilDatabase.Error())
	s.Equal("database: 无法提取 *gorm.DB，请确保使用 GORM 类型的数据库", ErrNotGORMDatabase.Error())
}

func (s *DatabaseTestSuite) TestConstants() {
	s.Equal("mysql", DriverMySQL)
	s.Equal("postgres", DriverPostgres)
	s.Equal("postgresql", DriverPostgreSQL)
	s.Equal("sqlite", DriverSQLite)
	s.Equal("sqlite3", DriverSQLite3)
	s.Equal("gorm", TypeGORM)
}

func (s *DatabaseTestSuite) TestDefaultConfig() {
	cfg := DefaultConfig()

	s.Equal(TypeGORM, cfg.Type)
	s.Equal(200*time.Millisecond, cfg.SlowThreshold)
	s.Equal("info", cfg.LogLevel)
	s.Equal(100, cfg.Pool.MaxOpen)
	s.Equal(10, cfg.Pool.MaxIdle)
	s.Equal(time.Hour, cfg.Pool.MaxLifetime)
	s.Equal(10*time.Minute, cfg.Pool.MaxIdleTime)
}

func (s *DatabaseTestSuite) TestDefaultPoolConfig() {
	pool := DefaultPoolConfig()

	s.Equal(100, pool.MaxOpen)
	s.Equal(10, pool.MaxIdle)
	s.Equal(time.Hour, pool.MaxLifetime)
	s.Equal(10*time.Minute, pool.MaxIdleTime)
}

func (s *DatabaseTestSuite) TestConfig_Validate() {
	tests := []struct {
		name    string
		config  *Config
		wantErr error
	}{
		{
			name:    "empty driver",
			config:  &Config{DSN: "test"},
			wantErr: ErrEmptyDriver,
		},
		{
			name:    "empty dsn",
			config:  &Config{Driver: DriverMySQL},
			wantErr: ErrEmptyDSN,
		},
		{
			name: "valid config",
			config: &Config{
				Driver: DriverMySQL,
				DSN:    "root:pass@tcp(localhost:3306)/test",
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			err := tt.config.Validate()
			if tt.wantErr != nil {
				s.ErrorIs(err, tt.wantErr)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *DatabaseTestSuite) TestConfig_ApplyDefaults() {
	cfg := &Config{}
	cfg.ApplyDefaults()

	s.Equal(TypeGORM, cfg.Type)
	s.Equal(200*time.Millisecond, cfg.SlowThreshold)
	s.Equal("info", cfg.LogLevel)
	s.Equal(100, cfg.Pool.MaxOpen)
	s.Equal(10, cfg.Pool.MaxIdle)
	s.Equal(time.Hour, cfg.Pool.MaxLifetime)
	s.Equal(10*time.Minute, cfg.Pool.MaxIdleTime)
}

func (s *DatabaseTestSuite) TestNewDatabase_NilConfig() {
	_, err := NewDatabase(nil, s.logger)
	s.ErrorIs(err, ErrNilConfig)
}

func (s *DatabaseTestSuite) TestNewDatabase_NilLogger() {
	cfg := &Config{Driver: DriverSQLite, DSN: ":memory:"}
	_, err := NewDatabase(cfg, nil)
	s.ErrorIs(err, ErrNilLogger)
}

func (s *DatabaseTestSuite) TestNewDatabase_EmptyDriver() {
	cfg := &Config{DSN: ":memory:"}
	_, err := NewDatabase(cfg, s.logger)
	s.ErrorIs(err, ErrEmptyDriver)
}

func (s *DatabaseTestSuite) TestNewDatabase_UnsupportedType() {
	cfg := &Config{
		Type:   "unknown",
		Driver: DriverSQLite,
		DSN:    ":memory:",
	}
	_, err := NewDatabase(cfg, s.logger)
	s.ErrorIs(err, ErrUnsupportedType)
}

func (s *DatabaseTestSuite) TestNewDatabase_UnsupportedDriver() {
	cfg := &Config{
		Type:   TypeGORM,
		Driver: "unknown",
		DSN:    "test",
	}
	_, err := NewDatabase(cfg, s.logger)
	s.ErrorIs(err, ErrUnsupportedDriver)
}

func (s *DatabaseTestSuite) TestNewDatabase_SQLite() {
	cfg := &Config{
		Driver: DriverSQLite,
		DSN:    ":memory:",
	}

	db, err := NewDatabase(cfg, s.logger)
	s.NoError(err)
	s.NotNil(db)
	defer db.Close()

	// 验证可以获取底层 DB
	s.NotNil(db.DB())
}

func (s *DatabaseTestSuite) TestNewDatabaseWithCleanup_SQLite() {
	cfg := &Config{
		Driver: DriverSQLite,
		DSN:    ":memory:",
	}

	db, cleanup, err := NewDatabaseWithCleanup(cfg, s.logger)
	s.NoError(err)
	s.NotNil(db)
	s.NotNil(cleanup)
	cleanup()
}

func (s *DatabaseTestSuite) TestMustNewDatabase_Success() {
	cfg := &Config{
		Driver: DriverSQLite,
		DSN:    ":memory:",
	}

	s.NotPanics(func() {
		db := MustNewDatabase(cfg, s.logger)
		s.NotNil(db)
		db.Close()
	})
}

func (s *DatabaseTestSuite) TestMustNewDatabase_Panic() {
	s.Panics(func() {
		MustNewDatabase(nil, s.logger)
	})
}

func (s *DatabaseTestSuite) TestAsGORM() {
	cfg := &Config{
		Driver: DriverSQLite,
		DSN:    ":memory:",
	}

	db, err := NewDatabase(cfg, s.logger)
	s.NoError(err)
	defer db.Close()

	gormDB := AsGORM(db)
	s.NotNil(gormDB)
}

func (s *DatabaseTestSuite) TestRequireGORM_NilDatabase() {
	gormDB, err := RequireGORM(nil)

	s.Nil(gormDB)
	s.ErrorIs(err, ErrNilDatabase)
}

func (s *DatabaseTestSuite) TestDB() {
	cfg := &Config{
		Driver: DriverSQLite,
		DSN:    ":memory:",
	}

	db, err := NewDatabase(cfg, s.logger)
	s.NoError(err)
	defer db.Close()

	ctx := s.T().Context()
	gormDB := DB(ctx, db)
	s.NotNil(gormDB)
}

func (s *DatabaseTestSuite) TestNewDatabase_WithTracing() {
	cfg := &Config{
		Driver:        DriverSQLite,
		DSN:           ":memory:",
		EnableTracing: true,
	}

	db, err := NewDatabase(cfg, s.logger)
	s.NoError(err)
	s.NotNil(db)
	defer db.Close()
}

// GORMTestSuite GORM 功能测试套件.
type GORMTestSuite struct {
	suite.Suite
	logger logger.Logger
	db     Database
}

func TestGORMSuite(t *testing.T) {
	suite.Run(t, new(GORMTestSuite))
}

func (s *GORMTestSuite) SetupSuite() {
	log, err := logger.NewLogger(logger.DefaultConfig())
	s.Require().NoError(err)
	s.logger = log

	cfg := &Config{
		Driver:      DriverSQLite,
		DSN:         ":memory:",
		AutoMigrate: true,
	}
	s.db, err = NewDatabase(cfg, s.logger)
	s.Require().NoError(err)
}

func (s *GORMTestSuite) TearDownSuite() {
	if s.db != nil {
		s.db.Close()
	}
	if s.logger != nil {
		s.logger.Close()
	}
}

type TestUser struct {
	BaseModel[uint]
	Name  string `gorm:"size:100"`
	Email string `gorm:"size:200;uniqueIndex"`
}

func (s *GORMTestSuite) TestAutoMigrate() {
	err := s.db.AutoMigrate(&TestUser{})
	s.NoError(err)

	// 验证表已创建
	gormDB := AsGORM(s.db)

	var count int64
	err = gormDB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='test_users'").Scan(&count).Error
	s.NoError(err)
	s.Equal(int64(1), count)
}

func (s *GORMTestSuite) TestAutoMigrate_Disabled() {
	cfg := &Config{
		Driver:      DriverSQLite,
		DSN:         ":memory:",
		AutoMigrate: false,
	}
	db, err := NewDatabase(cfg, s.logger)
	s.NoError(err)
	defer db.Close()

	// AutoMigrate 应该直接返回，不创建表
	err = db.AutoMigrate(&TestUser{})
	s.NoError(err)
}

func (s *GORMTestSuite) TestCRUD() {
	// 确保表存在
	err := s.db.AutoMigrate(&TestUser{})
	s.NoError(err)

	db := AsGORM(s.db)

	// Create
	user := &TestUser{Name: "John", Email: "john@example.com"}
	err = db.Create(user).Error
	s.NoError(err)
	s.NotZero(user.ID)

	// Read
	var found TestUser
	err = db.First(&found, user.ID).Error
	s.NoError(err)
	s.Equal("John", found.Name)

	// Update
	err = db.Model(&found).Update("name", "Jane").Error
	s.NoError(err)

	var updated TestUser
	err = db.First(&updated, user.ID).Error
	s.NoError(err)
	s.Equal("Jane", updated.Name)

	// Delete
	err = db.Delete(&updated).Error
	s.NoError(err)

	var deleted TestUser
	err = db.First(&deleted, user.ID).Error
	s.Error(err) // 应该找不到
}

func (s *GORMTestSuite) TestOnceMigratorDoRunsOnce() {
	var migrator OnceMigrator
	count := 0

	err := migrator.Do(func() error {
		count++
		return nil
	})
	s.NoError(err)

	err = migrator.Do(func() error {
		count++
		return nil
	})
	s.NoError(err)
	s.Equal(1, count)
}
