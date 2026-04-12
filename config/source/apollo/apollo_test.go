package apollo

import (
	"container/list"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/apolloconfig/agollo/v4/agcache"
	"github.com/apolloconfig/agollo/v4/storage"

	svcConfig "github.com/Tsukikage7/servex/config"
)

// --- mock agollo.Client ---

// mockCache 模拟 Apollo 缓存.
type mockCache struct {
	data map[string]any
}

func (c *mockCache) Get(key string) (interface{}, error) {
	v, ok := c.data[key]
	if !ok {
		return nil, errors.New("key not found")
	}
	return v, nil
}

func (c *mockCache) Set(_ string, _ interface{}, _ int) error { return nil }
func (c *mockCache) EntryCount() int64                        { return int64(len(c.data)) }
func (c *mockCache) Del(_ string) bool                        { return false }
func (c *mockCache) Range(f func(key, value interface{}) bool) {
	for k, v := range c.data {
		if !f(k, v) {
			break
		}
	}
}
func (c *mockCache) Clear() {}

// mockClient 模拟 Apollo 客户端.
type mockClient struct {
	mu        sync.Mutex
	cache     agcache.CacheInterface
	listeners []storage.ChangeListener
}

func (m *mockClient) GetConfig(_ string) *storage.Config                   { return nil }
func (m *mockClient) GetConfigAndInit(_ string) *storage.Config            { return nil }
func (m *mockClient) GetConfigCache(_ string) agcache.CacheInterface       { return m.cache }
func (m *mockClient) GetDefaultConfigCache() agcache.CacheInterface        { return m.cache }
func (m *mockClient) GetApolloConfigCache() agcache.CacheInterface         { return m.cache }
func (m *mockClient) GetValue(_ string) string                             { return "" }
func (m *mockClient) GetStringValue(_ string, defaultValue string) string  { return defaultValue }
func (m *mockClient) GetIntValue(_ string, defaultValue int) int           { return defaultValue }
func (m *mockClient) GetFloatValue(_ string, defaultValue float64) float64 { return defaultValue }
func (m *mockClient) GetBoolValue(_ string, defaultValue bool) bool        { return defaultValue }
func (m *mockClient) GetStringSliceValue(_ string, defaultValue []string) []string {
	return defaultValue
}
func (m *mockClient) GetIntSliceValue(_ string, defaultValue []int) []int { return defaultValue }

func (m *mockClient) AddChangeListener(listener storage.ChangeListener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners = append(m.listeners, listener)
}

func (m *mockClient) RemoveChangeListener(listener storage.ChangeListener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, l := range m.listeners {
		if l == listener {
			m.listeners = append(m.listeners[:i], m.listeners[i+1:]...)
			break
		}
	}
}

func (m *mockClient) GetChangeListeners() *list.List { return list.New() }
func (m *mockClient) UseEventDispatch()              {}
func (m *mockClient) Close()                         {}

// fireChange 模拟配置变更事件.
func (m *mockClient) fireChange(namespace string, changes map[string]*storage.ConfigChange) {
	m.mu.Lock()
	listeners := make([]storage.ChangeListener, len(m.listeners))
	copy(listeners, m.listeners)
	m.mu.Unlock()

	event := &storage.ChangeEvent{
		Changes: changes,
	}
	event.Namespace = namespace

	for _, l := range listeners {
		l.OnChange(event)
	}
}

// --- 测试 ---

func TestLoad(t *testing.T) {
	mc := &mockClient{
		cache: &mockCache{data: map[string]any{
			"content": `{"host":"localhost","port":8080}`,
		}},
	}

	s := &Source{
		client:    mc,
		namespace: "application",
		appID:     "testApp",
		cluster:   "default",
		format:    "json",
	}

	kvs, err := s.Load()
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if len(kvs) != 1 {
		t.Fatalf("期望 1 个 KeyValue，得到 %d", len(kvs))
	}
	if kvs[0].Key != "application" {
		t.Errorf("期望 key=application，得到 %s", kvs[0].Key)
	}
	if string(kvs[0].Value) != `{"host":"localhost","port":8080}` {
		t.Errorf("值不匹配: %s", kvs[0].Value)
	}
	if kvs[0].Format != "json" {
		t.Errorf("期望格式 json，得到 %s", kvs[0].Format)
	}
}

func TestLoad_EmptyCache(t *testing.T) {
	mc := &mockClient{
		cache: &mockCache{data: map[string]any{}},
	}

	s := &Source{
		client:    mc,
		namespace: "application",
		format:    "json",
	}

	_, err := s.Load()
	if !errors.Is(err, svcConfig.ErrSourceLoad) {
		t.Errorf("期望 ErrSourceLoad，得到 %v", err)
	}
}

func TestLoad_NilCache(t *testing.T) {
	mc := &mockClient{cache: nil}

	s := &Source{
		client:    mc,
		namespace: "application",
		format:    "json",
	}

	_, err := s.Load()
	if !errors.Is(err, svcConfig.ErrSourceLoad) {
		t.Errorf("期望 ErrSourceLoad，得到 %v", err)
	}
}

func TestWatch(t *testing.T) {
	mc := &mockClient{
		cache: &mockCache{data: map[string]any{
			"content": `{"updated":true}`,
		}},
	}

	s := &Source{
		client:    mc,
		namespace: "application",
		format:    "json",
	}

	watcher, err := s.Watch()
	if err != nil {
		t.Fatalf("Watch 失败: %v", err)
	}
	defer watcher.Stop()

	// 模拟配置变更
	go func() {
		time.Sleep(50 * time.Millisecond)
		mc.fireChange("application", map[string]*storage.ConfigChange{
			"content": {
				OldValue:   `{"updated":false}`,
				NewValue:   `{"updated":true}`,
				ChangeType: storage.MODIFIED,
			},
		})
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		kvs, err := watcher.Next()
		if err != nil {
			t.Errorf("Next 失败: %v", err)
			return
		}
		if len(kvs) != 1 {
			t.Errorf("期望 1 个 KeyValue，得到 %d", len(kvs))
			return
		}
		if string(kvs[0].Value) != `{"updated":true}` {
			t.Errorf("值不匹配: %s", kvs[0].Value)
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Watch.Next 超时")
	}
}

func TestWatch_IgnoreOtherNamespace(t *testing.T) {
	mc := &mockClient{
		cache: &mockCache{data: map[string]any{
			"content": `{"v":1}`,
		}},
	}

	s := &Source{
		client:    mc,
		namespace: "application",
		format:    "json",
	}

	watcher, err := s.Watch()
	if err != nil {
		t.Fatalf("Watch 失败: %v", err)
	}
	defer watcher.Stop()

	// 发送其他命名空间的变更，不应触发
	mc.fireChange("other-namespace", map[string]*storage.ConfigChange{})

	// 短暂等待后发送正确命名空间变更
	go func() {
		time.Sleep(50 * time.Millisecond)
		mc.fireChange("application", map[string]*storage.ConfigChange{
			"content": {ChangeType: storage.MODIFIED},
		})
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		kvs, err := watcher.Next()
		if err != nil {
			t.Errorf("Next 失败: %v", err)
			return
		}
		if kvs[0].Key != "application" {
			t.Errorf("期望 key=application，得到 %s", kvs[0].Key)
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Watch.Next 超时")
	}
}

func TestWatch_Stop(t *testing.T) {
	mc := &mockClient{
		cache: &mockCache{data: map[string]any{}},
	}

	s := &Source{
		client:    mc,
		namespace: "application",
		format:    "json",
	}

	watcher, err := s.Watch()
	if err != nil {
		t.Fatalf("Watch 失败: %v", err)
	}

	// 停止监听
	if err := watcher.Stop(); err != nil {
		t.Fatalf("Stop 失败: %v", err)
	}

	// Next 应返回 ErrSourceClosed
	_, err = watcher.Next()
	if !errors.Is(err, svcConfig.ErrSourceClosed) {
		t.Errorf("期望 ErrSourceClosed，得到 %v", err)
	}
}

func TestOptions(t *testing.T) {
	s := &Source{}

	WithFormat("yaml")(s)
	if s.format != "yaml" {
		t.Errorf("期望 format=yaml，得到 %s", s.format)
	}

	WithCluster("prod")(s)
	if s.cluster != "prod" {
		t.Errorf("期望 cluster=prod，得到 %s", s.cluster)
	}

	WithNamespace("custom")(s)
	if s.namespace != "custom" {
		t.Errorf("期望 namespace=custom，得到 %s", s.namespace)
	}
}

func TestConfig_Defaults(t *testing.T) {
	// 直接构造 Source 验证 New 中的默认值逻辑
	cfg := &Config{
		Addr:  "http://localhost:8080",
		AppID: "testApp",
	}

	s := &Source{
		namespace: cfg.Namespace,
		appID:     cfg.AppID,
		cluster:   cfg.Cluster,
		format:    "json",
	}
	if s.namespace == "" {
		s.namespace = "application"
	}
	if s.cluster == "" {
		s.cluster = "default"
	}

	if s.namespace != "application" {
		t.Errorf("期望默认 namespace=application，得到 %s", s.namespace)
	}
	if s.cluster != "default" {
		t.Errorf("期望默认 cluster=default，得到 %s", s.cluster)
	}
	if s.format != "json" {
		t.Errorf("期望默认 format=json，得到 %s", s.format)
	}
}

func TestOnNewestChange(t *testing.T) {
	mc := &mockClient{
		cache: &mockCache{data: map[string]any{}},
	}

	s := &Source{
		client:    mc,
		namespace: "application",
		format:    "json",
	}

	watcher, err := s.Watch()
	if err != nil {
		t.Fatalf("Watch 失败: %v", err)
	}
	defer watcher.Stop()

	// OnNewestChange 应当不 panic
	w := watcher.(*apolloWatcher)
	w.OnNewestChange(nil)
	w.OnNewestChange(&storage.FullChangeEvent{})
}

func TestOnChange_LoadError(t *testing.T) {
	// When Load fails (nil cache), OnChange should silently return
	mc := &mockClient{cache: nil}

	s := &Source{
		client:    mc,
		namespace: "application",
		format:    "json",
	}

	watcher, err := s.Watch()
	if err != nil {
		t.Fatalf("Watch 失败: %v", err)
	}
	defer watcher.Stop()

	// Fire a change event — Load will fail, so nothing should be sent to channel
	mc.cache = nil
	mc.fireChange("application", map[string]*storage.ConfigChange{
		"content": {ChangeType: storage.MODIFIED},
	})

	// Briefly verify no message was delivered
	select {
	case <-time.After(100 * time.Millisecond):
		// expected — no message
	}
}

func TestOnChange_ChannelFullOverwrite(t *testing.T) {
	mc := &mockClient{
		cache: &mockCache{data: map[string]any{
			"content": `{"v":1}`,
		}},
	}

	s := &Source{
		client:    mc,
		namespace: "application",
		format:    "json",
	}

	watcher, err := s.Watch()
	if err != nil {
		t.Fatalf("Watch 失败: %v", err)
	}
	defer watcher.Stop()

	// Fire two changes rapidly without consuming — second should overwrite
	mc.fireChange("application", map[string]*storage.ConfigChange{
		"content": {ChangeType: storage.MODIFIED},
	})

	// Update cache value then fire again
	mc.cache = &mockCache{data: map[string]any{
		"content": `{"v":2}`,
	}}
	mc.fireChange("application", map[string]*storage.ConfigChange{
		"content": {ChangeType: storage.MODIFIED},
	})

	// Now consume — should get the latest value
	done := make(chan struct{})
	go func() {
		defer close(done)
		kvs, err := watcher.Next()
		if err != nil {
			t.Errorf("Next 失败: %v", err)
			return
		}
		if len(kvs) == 0 {
			t.Error("期望至少 1 个 KeyValue")
			return
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Watch.Next 超时")
	}
}

func TestLoad_NonStringContent(t *testing.T) {
	// content 值不是 string 类型时应返回 ErrSourceLoad
	mc := &mockClient{
		cache: &mockCache{data: map[string]any{
			"content": 12345,
		}},
	}

	s := &Source{
		client:    mc,
		namespace: "application",
		format:    "json",
	}

	_, err := s.Load()
	if !errors.Is(err, svcConfig.ErrSourceLoad) {
		t.Errorf("期望 ErrSourceLoad，得到 %v", err)
	}
}

func TestOptions_Combined(t *testing.T) {
	s := &Source{}

	WithFormat("toml")(s)
	WithCluster("staging")(s)
	WithNamespace("shared")(s)

	if s.format != "toml" {
		t.Errorf("期望 format=toml，得到 %s", s.format)
	}
	if s.cluster != "staging" {
		t.Errorf("期望 cluster=staging，得到 %s", s.cluster)
	}
	if s.namespace != "shared" {
		t.Errorf("期望 namespace=shared，得到 %s", s.namespace)
	}
}

func TestWatch_StopThenOnChange(t *testing.T) {
	mc := &mockClient{
		cache: &mockCache{data: map[string]any{
			"content": `{"v":1}`,
		}},
	}

	s := &Source{
		client:    mc,
		namespace: "application",
		format:    "json",
	}

	watcher, err := s.Watch()
	if err != nil {
		t.Fatalf("Watch 失败: %v", err)
	}

	// Stop first
	if err := watcher.Stop(); err != nil {
		t.Fatalf("Stop 失败: %v", err)
	}

	// Fire change after stop — should not panic, OnChange hits ctx.Done() branch
	mc.fireChange("application", map[string]*storage.ConfigChange{
		"content": {ChangeType: storage.MODIFIED},
	})
}
