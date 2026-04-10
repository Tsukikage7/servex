// Package nacos 提供基于 Nacos 的配置源实现.
package nacos

import (
	"context"
	"log"

	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"

	"github.com/Tsukikage7/servex/config"
)

// Source Nacos 配置源.
type Source struct {
	client    config_client.IConfigClient
	dataID    string
	group     string
	namespace string
	format    string
}

// Option Nacos 配置源选项.
type Option func(*Source)

// WithFormat 指定配置格式，默认为 "json".
func WithFormat(format string) Option {
	return func(s *Source) {
		s.format = format
	}
}

// WithGroup 指定 Nacos 分组，默认为 "DEFAULT_GROUP".
func WithGroup(group string) Option {
	return func(s *Source) {
		s.group = group
	}
}

// WithNamespace 指定 Nacos 命名空间.
func WithNamespace(namespace string) Option {
	return func(s *Source) {
		s.namespace = namespace
	}
}

// New 创建 Nacos 配置源.
func New(client config_client.IConfigClient, dataID string, opts ...Option) *Source {
	s := &Source{
		client: client,
		dataID: dataID,
		group:  "DEFAULT_GROUP",
		format: "json",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Load 从 Nacos 读取配置.
func (s *Source) Load() ([]*config.KeyValue, error) {
	content, err := s.client.GetConfig(vo.ConfigParam{
		DataId: s.dataID,
		Group:  s.group,
	})
	if err != nil {
		return nil, err
	}
	if content == "" {
		return nil, config.ErrSourceLoad
	}
	return []*config.KeyValue{
		{
			Key:    s.dataID,
			Value:  []byte(content),
			Format: s.format,
		},
	}, nil
}

// Watch 创建基于 Nacos 监听的变更监听器.
func (s *Source) Watch() (config.Watcher, error) {
	ctx, cancel := context.WithCancel(context.Background())
	w := &nacosWatcher{
		source: s,
		ctx:    ctx,
		cancel: cancel,
		ch:     make(chan string, 1),
	}

	err := s.client.ListenConfig(vo.ConfigParam{
		DataId: s.dataID,
		Group:  s.group,
		OnChange: func(namespace, group, dataId, data string) {
			select {
			case w.ch <- data:
			default:
				// channel 满，丢弃本次变更通知（下次变更会携带最新配置）
				log.Printf("nacos: watcher channel 已满，丢弃变更通知 dataId=%s group=%s", dataId, group)
			}
		},
	})
	if err != nil {
		cancel()
		return nil, err
	}

	return w, nil
}

// nacosWatcher Nacos 变更监听器.
type nacosWatcher struct {
	source *Source
	ctx    context.Context
	cancel context.CancelFunc
	ch     chan string
}

// Next 阻塞直到 Nacos 配置发生变更.
func (w *nacosWatcher) Next() ([]*config.KeyValue, error) {
	select {
	case <-w.ctx.Done():
		return nil, config.ErrSourceClosed
	case data, ok := <-w.ch:
		if !ok {
			return nil, config.ErrSourceClosed
		}
		return []*config.KeyValue{
			{
				Key:    w.source.dataID,
				Value:  []byte(data),
				Format: w.source.format,
			},
		}, nil
	}
}

// Stop 停止 Nacos 监听.
func (w *nacosWatcher) Stop() error {
	w.cancel()
	return w.source.client.CancelListenConfig(vo.ConfigParam{
		DataId: w.source.dataID,
		Group:  w.source.group,
	})
}

// 编译期接口合规检查.
var _ config.Source = (*Source)(nil)
var _ config.Watcher = (*nacosWatcher)(nil)
