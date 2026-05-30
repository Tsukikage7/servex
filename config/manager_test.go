package config

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// ManagerTestSuite 配置管理器测试套件.
type ManagerTestSuite struct {
	suite.Suite
}

func TestManagerSuite(t *testing.T) {
	suite.Run(t, new(ManagerTestSuite))
}

// === 测试用 Source 实现 ===

// staticSource 静态配置源用于测试.
type staticSource struct {
	kvs     []*KeyValue
	loadErr error
	watcher *staticWatcher
}

func newStaticSource(format string, data []byte) *staticSource {
	return &staticSource{
		kvs: []*KeyValue{{Key: "test", Value: data, Format: format}},
	}
}

func (s *staticSource) Load() ([]*KeyValue, error) {
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	return s.kvs, nil
}

func (s *staticSource) Watch() (Watcher, error) {
	if s.watcher == nil {
		s.watcher = &staticWatcher{
			ch:     make(chan []*KeyValue, 1),
			stopCh: make(chan struct{}),
		}
	}
	return s.watcher, nil
}

// trigger 触发一次配置变更.
func (s *staticSource) trigger(data []byte) {
	s.kvs = []*KeyValue{{Key: "test", Value: data, Format: s.kvs[0].Format}}
	if s.watcher != nil {
		s.watcher.ch <- s.kvs
	}
}

// staticWatcher 静态监听器.
type staticWatcher struct {
	ch     chan []*KeyValue
	stopCh chan struct{}
}

func (w *staticWatcher) Next() ([]*KeyValue, error) {
	select {
	case <-w.stopCh:
		return nil, ErrSourceClosed
	case kvs := <-w.ch:
		return kvs, nil
	}
}

func (w *staticWatcher) Stop() error {
	select {
	case <-w.stopCh:
	default:
		close(w.stopCh)
	}
	return nil
}

// === 测试配置类型 ===

type managerTestConfig struct {
	Name string `json:"name" yaml:"name"`
	Port int    `json:"port" yaml:"port"`
}

type validatableManagerConfig struct {
	Name string `json:"name" yaml:"name"`
	Port int    `json:"port" yaml:"port"`
}

func (c *validatableManagerConfig) Validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return ErrValidation
	}
	return nil
}

// === NewManager 测试 ===

func (s *ManagerTestSuite) TestNewManager_NoSource() {
	_, err := NewManager[managerTestConfig]()
	s.Error(err)
}

func (s *ManagerTestSuite) TestNewManager_Success() {
	src := newStaticSource("json", []byte(`{"name":"test","port":8080}`))
	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](src),
	)
	s.NoError(err)
	s.NotNil(mgr)
}

// === Load 测试 ===

func (s *ManagerTestSuite) TestLoad_JSON() {
	src := newStaticSource("json", []byte(`{"name":"app","port":3000}`))
	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](src),
	)
	s.Require().NoError(err)

	err = mgr.Load()
	s.NoError(err)

	cfg := mgr.Get()
	s.NotNil(cfg)
	s.Equal("app", cfg.Name)
	s.Equal(3000, cfg.Port)
}

func (s *ManagerTestSuite) TestLoad_YAML() {
	data := []byte("name: yaml-app\nport: 4000\n")
	src := newStaticSource("yaml", data)
	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](src),
	)
	s.Require().NoError(err)

	err = mgr.Load()
	s.NoError(err)

	cfg := mgr.Get()
	s.Equal("yaml-app", cfg.Name)
	s.Equal(4000, cfg.Port)
}

func (s *ManagerTestSuite) TestLoad_SourceError() {
	src := &staticSource{loadErr: ErrSourceLoad}
	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](src),
	)
	s.Require().NoError(err)

	err = mgr.Load()
	s.Error(err)
}

// === Get 测试 ===

func (s *ManagerTestSuite) TestGet_BeforeLoad() {
	src := newStaticSource("json", []byte(`{"name":"test"}`))
	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](src),
	)
	s.Require().NoError(err)

	cfg := mgr.Get()
	s.Nil(cfg)
}

// === Watch 测试 ===

func (s *ManagerTestSuite) TestWatch_AutoUpdate() {
	src := newStaticSource("json", []byte(`{"name":"v1","port":8080}`))
	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](src),
	)
	s.Require().NoError(err)

	s.Require().NoError(mgr.Load())
	s.Require().NoError(mgr.Watch())
	defer mgr.Close()

	s.Equal("v1", mgr.Get().Name)

	// 触发变更
	src.trigger([]byte(`{"name":"v2","port":9090}`))

	// 等待更新
	s.Eventually(func() bool {
		cfg := mgr.Get()
		return cfg != nil && cfg.Name == "v2" && cfg.Port == 9090
	}, 2*time.Second, 50*time.Millisecond)
}

// === Observer 测试 ===

func (s *ManagerTestSuite) TestObserver_Notified() {
	src := newStaticSource("json", []byte(`{"name":"v1","port":8080}`))

	var mu sync.Mutex
	var oldCfg, newCfg *managerTestConfig

	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](src),
		WithObserver[managerTestConfig](func(old, new *managerTestConfig) {
			mu.Lock()
			defer mu.Unlock()
			oldCfg = old
			newCfg = new
		}),
	)
	s.Require().NoError(err)

	s.Require().NoError(mgr.Load())
	s.Require().NoError(mgr.Watch())
	defer mgr.Close()

	src.trigger([]byte(`{"name":"v2","port":9090}`))

	s.Eventually(func() bool {
		mu.Lock()
		defer mu.Unlock()
		return newCfg != nil && newCfg.Name == "v2"
	}, 2*time.Second, 50*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	s.Equal("v1", oldCfg.Name)
	s.Equal("v2", newCfg.Name)
}

// === Validatable 测试 ===

func (s *ManagerTestSuite) TestLoad_Validatable_Success() {
	src := newStaticSource("json", []byte(`{"name":"app","port":8080}`))
	mgr, err := NewManager[validatableManagerConfig](
		WithSource[validatableManagerConfig](src),
	)
	s.Require().NoError(err)

	err = mgr.Load()
	s.NoError(err)
	s.Equal(8080, mgr.Get().Port)
}

func (s *ManagerTestSuite) TestLoad_Validatable_Failure() {
	src := newStaticSource("json", []byte(`{"name":"app","port":-1}`))
	mgr, err := NewManager[validatableManagerConfig](
		WithSource[validatableManagerConfig](src),
	)
	s.Require().NoError(err)

	err = mgr.Load()
	s.Error(err)
	s.Nil(mgr.Get())
}

func (s *ManagerTestSuite) TestWatch_Validatable_SkipInvalid() {
	src := newStaticSource("json", []byte(`{"name":"app","port":8080}`))
	mgr, err := NewManager[validatableManagerConfig](
		WithSource[validatableManagerConfig](src),
	)
	s.Require().NoError(err)

	s.Require().NoError(mgr.Load())
	s.Require().NoError(mgr.Watch())
	defer mgr.Close()

	// 触发无效配置更新
	src.trigger([]byte(`{"name":"bad","port":-1}`))
	time.Sleep(200 * time.Millisecond)

	// 配置不应更新
	s.Equal(8080, mgr.Get().Port)
}

// === 多 Source 合并测试 ===

func (s *ManagerTestSuite) TestLoad_MultiSource_Merge() {
	src1 := newStaticSource("json", []byte(`{"name":"base","port":3000}`))
	src2 := newStaticSource("json", []byte(`{"port":9000}`))

	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](src1),
		WithSource[managerTestConfig](src2),
	)
	s.Require().NoError(err)

	s.Require().NoError(mgr.Load())

	cfg := mgr.Get()
	// src2 覆盖 port，但 JSON 不能部分覆盖非 map merge，
	// 实际行为：src2 将 name 设为零值
	s.Equal(9000, cfg.Port)
}

// === Close 测试 ===

func (s *ManagerTestSuite) TestClose() {
	src := newStaticSource("json", []byte(`{"name":"test"}`))
	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](src),
	)
	s.Require().NoError(err)

	s.Require().NoError(mgr.Load())
	s.Require().NoError(mgr.Watch())

	err = mgr.Close()
	s.NoError(err)

	// 重复 Close 不 panic
	err = mgr.Close()
	s.NoError(err)
}

// === 并发安全测试 ===

func (s *ManagerTestSuite) TestConcurrentGet() {
	src := newStaticSource("json", []byte(`{"name":"concurrent","port":8080}`))
	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](src),
	)
	s.Require().NoError(err)
	s.Require().NoError(mgr.Load())

	var wg sync.WaitGroup
	var readCount atomic.Int64

	for range 100 {
		wg.Go(func() {
			for range 100 {
				cfg := mgr.Get()
				if cfg != nil {
					readCount.Add(1)
				}
			}
		})
	}

	wg.Wait()
	s.Equal(int64(10000), readCount.Load())
}

// === 自定义 Decoder 测试 ===

func (s *ManagerTestSuite) TestWithDecoder() {
	src := newStaticSource("custom", []byte(`custom-data`))
	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](src),
		WithDecoder[managerTestConfig](func(kvs []*KeyValue) (*managerTestConfig, error) {
			return &managerTestConfig{Name: "decoded", Port: 1234}, nil
		}),
	)
	s.Require().NoError(err)

	s.Require().NoError(mgr.Load())
	s.Equal("decoded", mgr.Get().Name)
	s.Equal(1234, mgr.Get().Port)
}

// === defaultDecoder 分支测试 ===

func (s *ManagerTestSuite) TestLoad_UnknownFormat_FallbackJSON() {
	// 未知格式会尝试 JSON 解析
	src := newStaticSource("toml", []byte(`{"name":"fallback","port":5000}`))
	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](src),
	)
	s.Require().NoError(err)

	err = mgr.Load()
	s.NoError(err)
	s.Equal("fallback", mgr.Get().Name)
	s.Equal(5000, mgr.Get().Port)
}

func (s *ManagerTestSuite) TestLoad_UnknownFormat_InvalidJSON() {
	// 未知格式且不是有效 JSON
	src := newStaticSource("toml", []byte(`not valid json`))
	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](src),
	)
	s.Require().NoError(err)

	err = mgr.Load()
	s.Error(err)
}

func (s *ManagerTestSuite) TestLoad_InvalidJSON() {
	src := newStaticSource("json", []byte(`{bad json`))
	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](src),
	)
	s.Require().NoError(err)

	err = mgr.Load()
	s.Error(err)
}

func (s *ManagerTestSuite) TestLoad_InvalidYAML() {
	src := newStaticSource("yaml", []byte(":\n  :\n    - [invalid"))
	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](src),
	)
	s.Require().NoError(err)

	err = mgr.Load()
	s.Error(err)
}

func (s *ManagerTestSuite) TestLoad_YMLAlias() {
	// "yml" 应走 yaml 分支
	data := []byte("name: yml-app\nport: 7000\n")
	src := newStaticSource("yml", data)
	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](src),
	)
	s.Require().NoError(err)

	err = mgr.Load()
	s.NoError(err)
	s.Equal("yml-app", mgr.Get().Name)
}

// === Watch 创建失败测试 ===

// failWatchSource Watch 方法返回错误的配置源.
type failWatchSource struct {
	kvs []*KeyValue
}

func (s *failWatchSource) Load() ([]*KeyValue, error) {
	return s.kvs, nil
}

func (s *failWatchSource) Watch() (Watcher, error) {
	return nil, ErrSourceWatch
}

func (s *ManagerTestSuite) TestWatch_WatcherCreationFailure() {
	src := &failWatchSource{
		kvs: []*KeyValue{{Key: "test", Value: []byte(`{"name":"test"}`), Format: "json"}},
	}
	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](src),
	)
	s.Require().NoError(err)
	s.Require().NoError(mgr.Load())

	err = mgr.Watch()
	s.Error(err)
	s.ErrorIs(err, ErrSourceWatch)
}

func (s *ManagerTestSuite) TestWatch_PartialWatcherCreationFailure() {
	// 第一个源成功，第二个失败，应回滚关闭第一个
	goodSrc := newStaticSource("json", []byte(`{"name":"good"}`))
	badSrc := &failWatchSource{
		kvs: []*KeyValue{{Key: "bad", Value: []byte(`{"name":"bad"}`), Format: "json"}},
	}

	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](goodSrc),
		WithSource[managerTestConfig](badSrc),
	)
	s.Require().NoError(err)
	s.Require().NoError(mgr.Load())

	err = mgr.Watch()
	s.Error(err)
}

// === watchLoop 解码/加载错误分支 ===

func (s *ManagerTestSuite) TestWatch_DecodeError_SkipUpdate() {
	src := newStaticSource("json", []byte(`{"name":"v1","port":8080}`))
	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](src),
	)
	s.Require().NoError(err)

	s.Require().NoError(mgr.Load())
	s.Require().NoError(mgr.Watch())
	defer mgr.Close()

	// 触发无法解码的数据将源数据改为无效 JSON
	src.kvs = []*KeyValue{{Key: "test", Value: []byte(`{invalid`), Format: "json"}}
	src.watcher.ch <- src.kvs

	time.Sleep(200 * time.Millisecond)

	// 配置不应更新
	s.Equal("v1", mgr.Get().Name)
}

func (s *ManagerTestSuite) TestWatch_LoadAllError_SkipUpdate() {
	src := newStaticSource("json", []byte(`{"name":"v1","port":8080}`))
	mgr, err := NewManager[managerTestConfig](
		WithSource[managerTestConfig](src),
	)
	s.Require().NoError(err)

	s.Require().NoError(mgr.Load())
	s.Require().NoError(mgr.Watch())
	defer mgr.Close()

	// 触发变更，但让 loadAll 失败
	src.loadErr = ErrSourceLoad
	src.watcher.ch <- src.kvs

	time.Sleep(200 * time.Millisecond)

	// 配置不应更新
	s.Equal("v1", mgr.Get().Name)
}
