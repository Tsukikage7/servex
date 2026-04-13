package cache

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// CacheTestSuite 缓存接口测试套件.
type CacheTestSuite struct {
	suite.Suite
}

func TestCacheSuite(t *testing.T) {
	suite.Run(t, new(CacheTestSuite))
}

func (s *CacheTestSuite) TestErrors() {
	s.Equal("缓存键不存在", ErrNotFound.Error())
	s.Equal("锁未持有或已过期", ErrLockNotHeld.Error())
	s.Equal("缓存配置为空", ErrNilConfig.Error())
	s.Equal("缓存地址为空", ErrEmptyAddr.Error())
	s.Equal("不支持的缓存类型", ErrUnsupported.Error())
	s.Equal("日志记录器为空", ErrNilLogger.Error())
}

func (s *CacheTestSuite) TestConstants() {
	s.Equal("redis", TypeRedis)
	s.Equal("memory", TypeMemory)
}

// OptionsTestSuite 选项测试套件.
type OptionsTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func TestOptionsSuite(t *testing.T) {
	suite.Run(t, new(OptionsTestSuite))
}

func (s *OptionsTestSuite) SetupSuite() {
	log, err := logger.NewLogger(logger.DefaultConfig())
	s.Require().NoError(err)
	s.logger = log
}

func (s *OptionsTestSuite) TearDownSuite() {
	if s.logger != nil {
		s.logger.Close()
	}
}

func (s *OptionsTestSuite) TestNew_MemoryCache() {
	config := NewMemoryConfig()
	cache, err := NewCache(config, s.logger)
	s.NoError(err)
	s.NotNil(cache)
	defer cache.Close()
}

func (s *OptionsTestSuite) TestNew_InvalidConfig() {
	config := &Config{Type: "invalid"}
	_, err := NewCache(config, s.logger)
	s.Error(err)
}

func (s *OptionsTestSuite) TestNew_NilConfig() {
	_, err := NewCache(nil, s.logger)
	s.Error(err)
}

func (s *OptionsTestSuite) TestNew_NilLogger() {
	config := NewMemoryConfig()
	_, err := NewCache(config, nil)
	s.Error(err)
	s.Equal(ErrNilLogger, err)
}

func (s *OptionsTestSuite) TestNew_Unsupported() {
	// 手动构造一个绕过 Validate 的情况
	config := &Config{Type: ""}
	config.ApplyDefaults()
	// Type 被设置为 redis，但没有 addr
	_, err := NewCache(config, s.logger)
	s.Error(err)
}

func (s *OptionsTestSuite) TestMustNew_Success() {
	config := NewMemoryConfig()

	s.NotPanics(func() {
		cache := MustNewCache(config, s.logger)
		s.NotNil(cache)
		cache.Close()
	})
}

func (s *OptionsTestSuite) TestMustNew_Panic() {
	config := &Config{Type: "invalid"}

	s.Panics(func() {
		MustNewCache(config, s.logger)
	})
}

func (s *OptionsTestSuite) TestMustNew_NilLoggerPanic() {
	config := NewMemoryConfig()

	s.Panics(func() {
		MustNewCache(config, nil)
	})
}
