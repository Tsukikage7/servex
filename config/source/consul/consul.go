// Package consul 提供基于 Consul KV 的配置源实现.
package consul

import (
	"context"

	"github.com/hashicorp/consul/api"

	"github.com/Tsukikage7/servex/v2/config"
)

// Source Consul KV 配置源.
type Source struct {
	client     *api.Client
	key        string
	format     string
	datacenter string
}

// Option Consul 配置源选项.
type Option func(*Source)

// WithFormat 指定配置格式，默认为 "json".
func WithFormat(format string) Option {
	return func(s *Source) {
		s.format = format
	}
}

// WithDatacenter 指定 Consul 数据中心.
func WithDatacenter(dc string) Option {
	return func(s *Source) {
		s.datacenter = dc
	}
}

// New 创建 Consul KV 配置源.
func New(client *api.Client, key string, opts ...Option) *Source {
	s := &Source{
		client: client,
		key:    key,
		format: "json",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Load 从 Consul KV 读取配置.
func (s *Source) Load() ([]*config.KeyValue, error) {
	opts := s.queryOptions()
	pair, _, err := s.client.KV().Get(s.key, opts)
	if err != nil {
		return nil, err
	}
	if pair == nil {
		return nil, config.ErrSourceLoad
	}
	return []*config.KeyValue{
		{
			Key:    s.key,
			Value:  pair.Value,
			Format: s.format,
		},
	}, nil
}

// Watch 创建基于 blocking query 的变更监听器.
func (s *Source) Watch() (config.Watcher, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &consulWatcher{
		source: s,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// queryOptions 构建 Consul 查询选项.
func (s *Source) queryOptions() *api.QueryOptions {
	opts := &api.QueryOptions{}
	if s.datacenter != "" {
		opts.Datacenter = s.datacenter
	}
	return opts
}

// consulWatcher Consul 变更监听器.
type consulWatcher struct {
	source    *Source
	ctx       context.Context
	cancel    context.CancelFunc
	lastIndex uint64
}

// Next 阻塞直到 Consul KV 值变更.
// 使用 Consul blocking query（长轮询）实现.
func (w *consulWatcher) Next() ([]*config.KeyValue, error) {
	// 首次调用时先获取当前 index，避免 WaitIndex=0 导致虚假变更通知
	if w.lastIndex == 0 {
		opts := w.source.queryOptions()
		opts = opts.WithContext(w.ctx)
		_, meta, err := w.source.client.KV().Get(w.source.key, opts)
		if err != nil {
			if w.ctx.Err() != nil {
				return nil, config.ErrSourceClosed
			}
			return nil, err
		}
		if meta != nil {
			w.lastIndex = meta.LastIndex
		}
	}

	for {
		opts := w.source.queryOptions()
		opts.WaitIndex = w.lastIndex
		opts = opts.WithContext(w.ctx)

		pair, meta, err := w.source.client.KV().Get(w.source.key, opts)
		if err != nil {
			// 检查 context 取消
			if w.ctx.Err() != nil {
				return nil, config.ErrSourceClosed
			}
			return nil, err
		}

		// 更新索引
		if meta != nil && meta.LastIndex > w.lastIndex {
			w.lastIndex = meta.LastIndex
			if pair == nil {
				continue
			}
			return []*config.KeyValue{
				{
					Key:    w.source.key,
					Value:  pair.Value,
					Format: w.source.format,
				},
			}, nil
		}
	}
}

// Stop 停止 Consul 监听.
func (w *consulWatcher) Stop() error {
	w.cancel()
	return nil
}
