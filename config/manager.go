package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"gopkg.in/yaml.v3"
)

// Observer 配置变更观察者回调.
type Observer[T any] func(old, new *T)

// Namer 支持命名的数据源接口.
type Namer interface {
	Name() string
}

// Manager 配置管理器，支持多数据源 + 热加载.
type Manager[T any] struct {
	sources   []Source
	decoder   func([]*KeyValue) (*T, error)
	observers []Observer[T]
	current   atomic.Pointer[T]
	mu        sync.Mutex // 保护 watchers 的并发访问
	watchers  []Watcher
	wg        sync.WaitGroup
	closed    chan struct{}
}

// ManagerOption 管理器选项.
type ManagerOption[T any] func(*Manager[T])

// WithSource 添加配置数据源.
func WithSource[T any](src Source) ManagerOption[T] {
	return func(m *Manager[T]) {
		m.sources = append(m.sources, src)
	}
}

// WithDecoder 设置配置解码函数（KeyValue -> T）.
// 默认实现: 根据 KeyValue.Format 选择 json/yaml 解码.
func WithDecoder[T any](fn func([]*KeyValue) (*T, error)) ManagerOption[T] {
	return func(m *Manager[T]) {
		m.decoder = fn
	}
}

// WithObserver 添加配置变更观察者.
func WithObserver[T any](obs Observer[T]) ManagerOption[T] {
	return func(m *Manager[T]) {
		m.observers = append(m.observers, obs)
	}
}

// NewManager 创建配置管理器.
func NewManager[T any](opts ...ManagerOption[T]) (*Manager[T], error) {
	m := &Manager[T]{
		closed: make(chan struct{}),
	}
	for _, opt := range opts {
		opt(m)
	}
	if len(m.sources) == 0 {
		return nil, fmt.Errorf("config: %w: 至少需要一个数据源", ErrSourceLoad)
	}
	if m.decoder == nil {
		m.decoder = defaultDecoder[T]
	}
	return m, nil
}

// Load 从所有 Source 加载配置，合并并存储.
func (m *Manager[T]) Load() error {
	kvs, err := m.loadAll()
	if err != nil {
		return err
	}
	cfg, err := m.decoder(kvs)
	if err != nil {
		return &ConfigFieldError{
			Source:  m.sourceNames(),
			Message: fmt.Sprintf("%v: %v", ErrUnmarshal, err),
			Err:     ErrUnmarshal,
		}
	}
	// 验证
	if v, ok := any(cfg).(Validatable); ok {
		if err := v.Validate(); err != nil {
			return &ConfigFieldError{
				Source:  m.sourceNames(),
				Message: fmt.Sprintf("%v: %v", ErrValidation, err),
				Err:     ErrValidation,
			}
		}
	}
	m.current.Store(cfg)
	return nil
}

// Watch 启动所有 Source 的 Watcher，检测变更时自动重新加载.
// 非阻塞，内部启动 goroutine.
func (m *Manager[T]) Watch() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, src := range m.sources {
		w, err := src.Watch()
		if err != nil {
			// 关闭已创建的 watcher
			for _, existing := range m.watchers {
				existing.Stop()
			}
			return fmt.Errorf("config: %w: %v", ErrSourceWatch, err)
		}
		m.watchers = append(m.watchers, w)
		m.wg.Go(func() { m.watchLoop(w) })
	}
	return nil
}

// Get 获取当前配置（无锁，atomic.Pointer.Load）.
func (m *Manager[T]) Get() *T {
	return m.current.Load()
}

// Close 停止所有 Watcher.
func (m *Manager[T]) Close() error {
	select {
	case <-m.closed:
		return nil
	default:
		close(m.closed)
	}
	m.mu.Lock()
	watchers := make([]Watcher, len(m.watchers))
	copy(watchers, m.watchers)
	m.mu.Unlock()

	for _, w := range watchers {
		w.Stop()
	}
	m.wg.Wait()
	return nil
}

// sourceNames 返回所有数据源名称的汇总字符串.
func (m *Manager[T]) sourceNames() string {
	names := make([]string, 0, len(m.sources))
	for i, src := range m.sources {
		if n, ok := src.(Namer); ok {
			names = append(names, n.Name())
		} else {
			names = append(names, fmt.Sprintf("source[%d]", i))
		}
	}
	if len(names) == 1 {
		return names[0]
	}
	result := names[0]
	for _, n := range names[1:] {
		result += "," + n
	}
	return result
}

// loadAll 从所有 Source 加载并合并 KeyValue.
// 后者覆盖前者.
func (m *Manager[T]) loadAll() ([]*KeyValue, error) {
	var all []*KeyValue
	for _, src := range m.sources {
		kvs, err := src.Load()
		if err != nil {
			return nil, fmt.Errorf("config: %w: %v", ErrSourceLoad, err)
		}
		all = append(all, kvs...)
	}
	return all, nil
}

// watchLoop 单个 Watcher 的监听循环.
func (m *Manager[T]) watchLoop(w Watcher) {
	for {
		select {
		case <-m.closed:
			return
		default:
		}

		_, err := w.Next()
		if err != nil {
			// watcher 被关闭或出错，退出
			return
		}

		// 重新从所有源加载（确保合并一致性）
		kvs, err := m.loadAll()
		if err != nil {
			slog.Error("[config] 热加载失败: 加载数据源出错", slog.Any("error", err))
			continue
		}
		cfg, err := m.decoder(kvs)
		if err != nil {
			slog.Error("[config] 热加载失败: 解码配置出错", slog.Any("error", err))
			continue
		}

		// 验证
		if v, ok := any(cfg).(Validatable); ok {
			if err := v.Validate(); err != nil {
				slog.Error("[config] 热加载失败: 配置验证出错", slog.Any("error", err))
				continue
			}
		}

		old := m.current.Swap(cfg)
		// 通知观察者（panic recovery 保护，避免单个观察者崩溃影响其他观察者）
		for _, obs := range m.observers {
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("[config] 观察者回调 panic", slog.Any("recover", r))
					}
				}()
				obs(old, cfg)
			}()
		}
	}
}

// defaultDecoder 默认解码器，根据 Format 选择 json/yaml 解码.
// 多个 KeyValue 时，后者覆盖前者（先解码到同一结构）.
//
// 注意: json.Unmarshal 对 slice 字段采用追加语义而非覆盖，
// 若需要覆盖行为请使用自定义 decoder（通过 WithDecoder 设置）.
func defaultDecoder[T any](kvs []*KeyValue) (*T, error) {
	cfg := new(T)
	for _, kv := range kvs {
		switch kv.Format {
		case "json":
			if err := json.Unmarshal(kv.Value, cfg); err != nil {
				return nil, err
			}
		case "yaml", "yml":
			if err := yaml.Unmarshal(kv.Value, cfg); err != nil {
				return nil, err
			}
		default:
			// 未知格式尝试 JSON
			if err := json.Unmarshal(kv.Value, cfg); err != nil {
				return nil, fmt.Errorf("unsupported format %q: %w", kv.Format, err)
			}
		}
	}
	return cfg, nil
}
