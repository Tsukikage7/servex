// Package file 提供基于本地文件的配置源实现.
package file

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/Tsukikage7/servex/v2/config"
)

// Source 文件配置源.
type Source struct {
	path   string
	format string
}

// Option 文件配置源选项.
type Option func(*Source)

// WithFormat 显式指定配置格式，默认从扩展名推断.
func WithFormat(format string) Option {
	return func(s *Source) {
		s.format = format
	}
}

// New 创建文件配置源.
func New(path string, opts ...Option) *Source {
	s := &Source{path: path}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Load 读取文件内容.
func (s *Source) Load() ([]*config.KeyValue, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	return []*config.KeyValue{
		{
			Key:    s.path,
			Value:  data,
			Format: s.resolveFormat(),
		},
	}, nil
}

// Watch 使用 fsnotify 监听文件变更.
func (s *Source) Watch() (config.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	// 监听文件所在目录，因为编辑器可能 rename+create 而非直接 write
	dir := filepath.Dir(s.path)
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return nil, err
	}
	return &fileWatcher{
		source:  s,
		watcher: watcher,
		stopCh:  make(chan struct{}),
	}, nil
}

// resolveFormat 解析配置格式.
func (s *Source) resolveFormat() string {
	if s.format != "" {
		return s.format
	}
	return inferFormat(s.path)
}

// inferFormat 从文件扩展名推断格式.
func inferFormat(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".toml":
		return "toml"
	default:
		return ""
	}
}

// fileWatcher 文件变更监听器.
type fileWatcher struct {
	source  *Source
	watcher *fsnotify.Watcher
	stopCh  chan struct{}
}

// Next 阻塞直到文件变更.
// 内置 100ms 去抖，合并连续写入事件.
func (w *fileWatcher) Next() ([]*config.KeyValue, error) {
	// 去抖定时器
	var debounce <-chan time.Time
	baseName := filepath.Base(w.source.path)

	for {
		select {
		case <-w.stopCh:
			return nil, config.ErrSourceClosed
		case event, ok := <-w.watcher.Events:
			if !ok {
				return nil, config.ErrSourceClosed
			}
			// 仅关注目标文件的写入/创建/删除事件
			if filepath.Base(event.Name) != baseName {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) == 0 {
				continue
			}
			// 重置去抖定时器
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

// Stop 停止文件监听.
func (w *fileWatcher) Stop() error {
	select {
	case <-w.stopCh:
		// 已关闭
	default:
		close(w.stopCh)
	}
	return w.watcher.Close()
}
