// Package k8s 提供基于 Kubernetes ConfigMap/Secret 的配置源实现.
package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/Tsukikage7/servex/config"
)

// ResourceType Kubernetes 资源类型.
type ResourceType int

const (
	// ResourceConfigMap ConfigMap 资源类型.
	ResourceConfigMap ResourceType = iota
	// ResourceSecret Secret 资源类型.
	ResourceSecret
)

// Source Kubernetes ConfigMap/Secret 配置源.
type Source struct {
	clientset    kubernetes.Interface
	namespace    string
	name         string
	resourceType ResourceType
	key          string
	format       string
}

// Option Kubernetes 配置源选项.
type Option func(*Source)

// WithFormat 指定配置格式，默认为 "json".
func WithFormat(format string) Option {
	return func(s *Source) {
		s.format = format
	}
}

// WithNamespace 指定 Kubernetes 命名空间，默认为 "default".
func WithNamespace(namespace string) Option {
	return func(s *Source) {
		s.namespace = namespace
	}
}

// WithKey 指定 ConfigMap/Secret data 中的具体键名.
// 若未指定，则返回 data 中所有键值对.
func WithKey(key string) Option {
	return func(s *Source) {
		s.key = key
	}
}

// WithResourceType 指定资源类型（ConfigMap 或 Secret），默认为 ConfigMap.
func WithResourceType(rt ResourceType) Option {
	return func(s *Source) {
		s.resourceType = rt
	}
}

// Config Kubernetes 配置源连接配置.
type Config struct {
	// KubeconfigPath kubeconfig 文件路径，为空则使用 in-cluster 配置.
	KubeconfigPath string
	// Namespace Kubernetes 命名空间，默认为 "default".
	Namespace string
	// Name ConfigMap 或 Secret 的名称.
	Name string
	// ResourceType 资源类型（ConfigMap 或 Secret），默认为 ConfigMap.
	ResourceType ResourceType
	// Key ConfigMap/Secret data 中的具体键名，为空则返回所有键值对.
	Key string
	// Format 配置格式，默认为 "json".
	Format string
}

// New 创建 Kubernetes ConfigMap/Secret 配置源.
func New(cfg *Config, opts ...Option) (*Source, error) {
	if cfg == nil {
		return nil, config.ErrNilConfig
	}
	if cfg.Name == "" {
		return nil, fmt.Errorf("k8s config source: %w: name is required", config.ErrNilConfig)
	}

	namespace := cfg.Namespace
	if namespace == "" {
		namespace = "default"
	}

	s := &Source{
		namespace:    namespace,
		name:         cfg.Name,
		resourceType: cfg.ResourceType,
		key:          cfg.Key,
		format:       cfg.Format,
	}
	if s.format == "" {
		s.format = "json"
	}

	for _, opt := range opts {
		opt(s)
	}

	var restConfig *rest.Config
	var err error

	if cfg.KubeconfigPath != "" {
		restConfig, err = clientcmd.BuildConfigFromFlags("", cfg.KubeconfigPath)
	} else {
		restConfig, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("k8s config source: build rest config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("k8s config source: create clientset: %w", err)
	}

	s.clientset = clientset
	return s, nil
}

// NewWithClient 使用已有的 kubernetes.Interface 创建配置源（便于测试）.
func NewWithClient(clientset kubernetes.Interface, name string, opts ...Option) *Source {
	s := &Source{
		clientset:    clientset,
		namespace:    "default",
		name:         name,
		resourceType: ResourceConfigMap,
		format:       "json",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Load 从 Kubernetes ConfigMap 或 Secret 读取配置.
func (s *Source) Load() ([]*config.KeyValue, error) {
	data, err := s.getData(context.Background())
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, config.ErrSourceLoad
	}

	// 若指定了 key，只返回该 key 的值
	if s.key != "" {
		v, ok := data[s.key]
		if !ok {
			return nil, fmt.Errorf("k8s config source: key %q not found: %w", s.key, config.ErrSourceLoad)
		}
		return []*config.KeyValue{
			{
				Key:    s.key,
				Value:  []byte(v),
				Format: s.format,
			},
		}, nil
	}

	// 返回所有键值对
	kvs := make([]*config.KeyValue, 0, len(data))
	for k, v := range data {
		kvs = append(kvs, &config.KeyValue{
			Key:    k,
			Value:  []byte(v),
			Format: s.format,
		})
	}
	return kvs, nil
}

// Watch 创建基于 Kubernetes Watch API 的变更监听器.
func (s *Source) Watch() (config.Watcher, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &k8sWatcher{
		source: s,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// getData 从 Kubernetes API 读取资源数据.
func (s *Source) getData(ctx context.Context) (map[string]string, error) {
	switch s.resourceType {
	case ResourceConfigMap:
		cm, err := s.clientset.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("k8s config source: get configmap %s/%s: %w", s.namespace, s.name, err)
		}
		return cm.Data, nil
	case ResourceSecret:
		secret, err := s.clientset.CoreV1().Secrets(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("k8s config source: get secret %s/%s: %w", s.namespace, s.name, err)
		}
		data := make(map[string]string, len(secret.Data))
		for k, v := range secret.Data {
			data[k] = string(v)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("k8s config source: unsupported resource type: %d", s.resourceType)
	}
}

// k8sWatcher Kubernetes 变更监听器.
type k8sWatcher struct {
	source *Source
	ctx    context.Context
	cancel context.CancelFunc
}

// Next 阻塞直到 Kubernetes 资源发生变更.
func (w *k8sWatcher) Next() ([]*config.KeyValue, error) {
	watcher, err := w.startWatch()
	if err != nil {
		return nil, err
	}
	defer watcher.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return nil, config.ErrSourceClosed
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return nil, config.ErrSourceClosed
			}
			if event.Type == watch.Modified {
				return w.extractData(event.Object)
			}
		}
	}
}

// Stop 停止 Kubernetes 监听.
func (w *k8sWatcher) Stop() error {
	w.cancel()
	return nil
}

// startWatch 启动 Kubernetes Watch.
func (w *k8sWatcher) startWatch() (watch.Interface, error) {
	opts := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", w.source.name),
	}
	switch w.source.resourceType {
	case ResourceConfigMap:
		return w.source.clientset.CoreV1().ConfigMaps(w.source.namespace).Watch(w.ctx, opts)
	case ResourceSecret:
		return w.source.clientset.CoreV1().Secrets(w.source.namespace).Watch(w.ctx, opts)
	default:
		return nil, fmt.Errorf("k8s config source: unsupported resource type: %d", w.source.resourceType)
	}
}

// extractData 从 Watch 事件对象中提取配置数据.
func (w *k8sWatcher) extractData(obj interface{}) ([]*config.KeyValue, error) {
	var data map[string]string

	switch o := obj.(type) {
	case *corev1.ConfigMap:
		data = o.Data
	case *corev1.Secret:
		data = make(map[string]string, len(o.Data))
		for k, v := range o.Data {
			data[k] = string(v)
		}
	default:
		return nil, fmt.Errorf("k8s config source: unexpected object type: %T", obj)
	}

	if w.source.key != "" {
		v, ok := data[w.source.key]
		if !ok {
			return nil, fmt.Errorf("k8s config source: key %q not found: %w", w.source.key, config.ErrSourceLoad)
		}
		return []*config.KeyValue{
			{
				Key:    w.source.key,
				Value:  []byte(v),
				Format: w.source.format,
			},
		}, nil
	}

	kvs := make([]*config.KeyValue, 0, len(data))
	for k, v := range data {
		kvs = append(kvs, &config.KeyValue{
			Key:    k,
			Value:  []byte(v),
			Format: w.source.format,
		})
	}
	return kvs, nil
}

// 编译期接口合规检查.
var _ config.Source = (*Source)(nil)
var _ config.Watcher = (*k8sWatcher)(nil)
