// Package env 提供基于环境变量的配置源实现.
package env

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/Tsukikage7/servex/v2/config"
)

// Source 环境变量配置源.
type Source struct {
	prefix  string
	envFile string
	format  string
}

// Option 环境变量配置源选项.
type Option func(*Source)

// WithPrefix 仅读取指定前缀的环境变量前缀将被去除.
func WithPrefix(prefix string) Option {
	return func(s *Source) {
		s.prefix = prefix
	}
}

// WithEnvFile 指定 .env 文件路径，Watch 时监听文件变化.
func WithEnvFile(path string) Option {
	return func(s *Source) {
		s.envFile = path
	}
}

// New 创建环境变量配置源.
func New(opts ...Option) *Source {
	s := &Source{format: "json"}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Load 读取环境变量并序列化为 JSON 格式的 KeyValue.
func (s *Source) Load() ([]*config.KeyValue, error) {
	data, err := s.loadEnv()
	if err != nil {
		return nil, err
	}
	return []*config.KeyValue{
		{
			Key:    "env",
			Value:  data,
			Format: s.format,
		},
	}, nil
}

// Watch 监听配置变更.
// 若设置了 EnvFile，使用 fsnotify 监听文件变化.
// 否则返回不支持 Watch 的 watcherNext 直接返回 ErrSourceClosed.
func (s *Source) Watch() (config.Watcher, error) {
	if s.envFile == "" {
		return &noopWatcher{}, nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// 监听文件所在目录
	dir := filepath.Dir(s.envFile)
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return nil, err
	}

	return &envWatcher{
		source:  s,
		watcher: watcher,
		stopCh:  make(chan struct{}),
	}, nil
}

// loadEnv 从 os.Environ() 读取环境变量，应用前缀过滤，序列化为 JSON.
func (s *Source) loadEnv() ([]byte, error) {
	data := make(map[string]string)

	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]

		if s.prefix != "" {
			if !strings.HasPrefix(key, s.prefix) {
				continue
			}
			// 去除前缀
			key = strings.TrimPrefix(key, s.prefix)
		}

		if key == "" {
			continue
		}
		data[key] = value
	}

	return json.Marshal(data)
}

// envWatcher 基于 fsnotify 的环境变量文件监听器.
type envWatcher struct {
	source  *Source
	watcher *fsnotify.Watcher
	stopCh  chan struct{}
}

// Next 阻塞直到 .env 文件变更.
func (w *envWatcher) Next() ([]*config.KeyValue, error) {
	var debounce <-chan time.Time
	baseName := filepath.Base(w.source.envFile)

	for {
		select {
		case <-w.stopCh:
			return nil, config.ErrSourceClosed
		case event, ok := <-w.watcher.Events:
			if !ok {
				return nil, config.ErrSourceClosed
			}
			if filepath.Base(event.Name) != baseName {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) == 0 {
				continue
			}
			debounce = time.After(100 * time.Millisecond)
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return nil, config.ErrSourceClosed
			}
			return nil, err
		case <-debounce:
			return w.source.Load()
		}
	}
}

// Stop 停止监听.
func (w *envWatcher) Stop() error {
	select {
	case <-w.stopCh:
	default:
		close(w.stopCh)
	}
	return w.watcher.Close()
}

// noopWatcher 不支持 Watch 的空 watcher.
type noopWatcher struct{}

// Next 直接返回 ErrSourceClosed，表示不支持 Watch.
func (w *noopWatcher) Next() ([]*config.KeyValue, error) {
	return nil, config.ErrSourceClosed
}

// Stop 空操作.
func (w *noopWatcher) Stop() error { return nil }

// 编译期接口合规检查.
var _ config.Source = (*Source)(nil)
var _ config.Watcher = (*envWatcher)(nil)
var _ config.Watcher = (*noopWatcher)(nil)
