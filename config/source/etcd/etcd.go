// Package etcd 提供基于 etcd KV 的配置源实现.
package etcd

import (
	"context"
	"time"

	mvccpb "go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/Tsukikage7/servex/config"
)

// Source etcd KV 配置源.
type Source struct {
	client *clientv3.Client
	key    string
	format string
}

// Option etcd 配置源选项.
type Option func(*Source)

// WithFormat 指定配置格式，默认为 "json".
func WithFormat(format string) Option {
	return func(s *Source) {
		s.format = format
	}
}

// New 创建 etcd KV 配置源.
func New(client *clientv3.Client, key string, opts ...Option) *Source {
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

// Load 从 etcd KV 读取配置.
func (s *Source) Load() ([]*config.KeyValue, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := s.client.Get(ctx, s.key)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, config.ErrSourceLoad
	}
	return []*config.KeyValue{
		{
			Key:    s.key,
			Value:  resp.Kvs[0].Value,
			Format: s.format,
		},
	}, nil
}

// Watch 创建基于 etcd Watch 的变更监听器.
func (s *Source) Watch() (config.Watcher, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &etcdWatcher{
		source: s,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// etcdWatcher etcd 变更监听器.
type etcdWatcher struct {
	source *Source
	ctx    context.Context
	cancel context.CancelFunc
}

// Next 阻塞直到 etcd key 发生变更.
func (w *etcdWatcher) Next() ([]*config.KeyValue, error) {
	watchCh := w.source.client.Watch(w.ctx, w.source.key)

	for {
		select {
		case <-w.ctx.Done():
			return nil, config.ErrSourceClosed
		case resp, ok := <-watchCh:
			if !ok {
				return nil, config.ErrSourceClosed
			}
			if resp.Err() != nil {
				return nil, resp.Err()
			}

			// 仅处理 PUT（创建/更新）事件
			for _, event := range resp.Events {
				if event.Type == mvccpb.PUT {
					return []*config.KeyValue{
						{
							Key:    w.source.key,
							Value:  event.Kv.Value,
							Format: w.source.format,
						},
					}, nil
				}
			}
		}
	}
}

// Stop 停止 etcd 监听.
func (w *etcdWatcher) Stop() error {
	w.cancel()
	return nil
}

// 编译期接口合规检查.
var _ config.Source = (*Source)(nil)
var _ config.Watcher = (*etcdWatcher)(nil)
