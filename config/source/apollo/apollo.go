// Package apollo 提供基于 Apollo 配置中心的配置源实现.
package apollo

import (
	"context"
	"fmt"

	"github.com/apolloconfig/agollo/v4"
	agolloConfig "github.com/apolloconfig/agollo/v4/env/config"
	"github.com/apolloconfig/agollo/v4/storage"

	"github.com/Tsukikage7/servex/config"
)

// Source Apollo 配置中心配置源.
type Source struct {
	client    agollo.Client
	namespace string
	appID     string
	cluster   string
	format    string
}

// Option Apollo 配置源选项.
type Option func(*Source)

// WithFormat 指定配置格式，默认为 "json".
func WithFormat(format string) Option {
	return func(s *Source) {
		s.format = format
	}
}

// WithCluster 指定 Apollo 集群，默认为 "default".
func WithCluster(cluster string) Option {
	return func(s *Source) {
		s.cluster = cluster
	}
}

// WithNamespace 指定 Apollo 命名空间，默认为 "application".
func WithNamespace(namespace string) Option {
	return func(s *Source) {
		s.namespace = namespace
	}
}

// Config Apollo 配置中心连接配置.
type Config struct {
	Addr      string // Apollo 配置服务地址，如 "http://localhost:8080".
	AppID     string // Apollo 应用 ID.
	Cluster   string // Apollo 集群名，默认为 "default".
	Namespace string // Apollo 命名空间，默认为 "application".
	Secret    string // Apollo 访问密钥，可选.
}

// New 创建 Apollo 配置中心配置源.
func New(cfg *Config, opts ...Option) (*Source, error) {
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

	for _, opt := range opts {
		opt(s)
	}

	appCfg := &agolloConfig.AppConfig{
		AppID:          s.appID,
		Cluster:        s.cluster,
		IP:             cfg.Addr,
		NamespaceName:  s.namespace,
		IsBackupConfig: false,
		Secret:         cfg.Secret,
	}

	client, err := agollo.StartWithConfig(func() (*agolloConfig.AppConfig, error) {
		return appCfg, nil
	})
	if err != nil {
		return nil, fmt.Errorf("apollo: 启动客户端失败: %w", err)
	}

	s.client = client
	return s, nil
}

// Load 从 Apollo 配置中心读取配置.
func (s *Source) Load() ([]*config.KeyValue, error) {
	cache := s.client.GetConfigCache(s.namespace)
	if cache == nil {
		return nil, config.ErrSourceLoad
	}

	// 对于非 properties 格式的命名空间（如 yaml、json），
	// Apollo 将完整内容存储在 "content" 键下.
	value, err := cache.Get("content")
	if err != nil {
		return nil, config.ErrSourceLoad
	}

	content, ok := value.(string)
	if !ok {
		return nil, config.ErrSourceLoad
	}

	return []*config.KeyValue{
		{
			Key:    s.namespace,
			Value:  []byte(content),
			Format: s.format,
		},
	}, nil
}

// Watch 创建基于 Apollo 变更监听的配置监听器.
func (s *Source) Watch() (config.Watcher, error) {
	ctx, cancel := context.WithCancel(context.Background())
	w := &apolloWatcher{
		source: s,
		ctx:    ctx,
		cancel: cancel,
		ch:     make(chan []*config.KeyValue, 1),
	}

	s.client.AddChangeListener(w)
	return w, nil
}

// apolloWatcher Apollo 变更监听器.
type apolloWatcher struct {
	source *Source
	ctx    context.Context
	cancel context.CancelFunc
	ch     chan []*config.KeyValue
}

// OnChange 当配置发生变更时触发.
func (w *apolloWatcher) OnChange(event *storage.ChangeEvent) {
	if event.Namespace != w.source.namespace {
		return
	}

	// 重新加载完整配置
	kvs, err := w.source.Load()
	if err != nil {
		return
	}

	select {
	case <-w.ctx.Done():
	case w.ch <- kvs:
	default:
		// 丢弃旧值，写入新值
		select {
		case <-w.ch:
		default:
		}
		select {
		case w.ch <- kvs:
		default:
		}
	}
}

// OnNewestChange 当收到完整配置快照时触发.
func (w *apolloWatcher) OnNewestChange(_ *storage.FullChangeEvent) {}

// Next 阻塞直到 Apollo 配置变更.
func (w *apolloWatcher) Next() ([]*config.KeyValue, error) {
	select {
	case <-w.ctx.Done():
		return nil, config.ErrSourceClosed
	case kvs := <-w.ch:
		return kvs, nil
	}
}

// Stop 停止 Apollo 监听.
func (w *apolloWatcher) Stop() error {
	w.source.client.RemoveChangeListener(w)
	w.cancel()
	return nil
}

// 编译期接口合规检查.
var _ config.Source = (*Source)(nil)
var _ config.Watcher = (*apolloWatcher)(nil)
var _ storage.ChangeListener = (*apolloWatcher)(nil)
