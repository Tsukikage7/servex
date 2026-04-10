package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/Tsukikage7/servex/observability/logger"
	"github.com/Tsukikage7/servex/transport"
)

const (
	// etcdServiceKeyPrefix 服务注册 key 前缀.
	etcdServiceKeyPrefix = "/services/"
	// defaultEtcdLeaseTTL 默认租约 TTL（秒）.
	defaultEtcdLeaseTTL = 10
	// defaultEtcdDialTimeout 默认连接超时.
	defaultEtcdDialTimeout = 5 * time.Second
)

// etcdServiceInfo etcd 中存储的服务信息.
type etcdServiceInfo struct {
	Address  string   `json:"address"`
	Protocol string   `json:"protocol"`
	Version  string   `json:"version"`
	Tags     []string `json:"tags,omitzero"`
}

// etcdDiscovery 基于 etcd 的服务发现实现.
type etcdDiscovery struct {
	client  *clientv3.Client
	config  *Config
	logger  logger.Logger
	mu      sync.Mutex
	leases  map[string]clientv3.LeaseID   // serviceID -> leaseID
	cancels map[string]context.CancelFunc // serviceID -> keepalive cancel
}

// 编译期接口合规检查.
var _ Discovery = (*etcdDiscovery)(nil)

// newEtcdDiscovery 创建 etcd 服务发现实例.
func newEtcdDiscovery(config *Config, log logger.Logger) (Discovery, error) {
	endpoints := config.EtcdEndpoints
	if len(endpoints) == 0 {
		endpoints = []string{"127.0.0.1:2379"}
	}

	dialTimeout := config.EtcdDialTimeout
	if dialTimeout == 0 {
		dialTimeout = defaultEtcdDialTimeout
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		log.With(logger.Err(err)).Error("[Discovery] 创建etcd客户端失败")
		return nil, ErrClientCreate
	}

	return &etcdDiscovery{
		client:  client,
		config:  config,
		logger:  log,
		leases:  make(map[string]clientv3.LeaseID),
		cancels: make(map[string]context.CancelFunc),
	}, nil
}

// Register 注册服务实例，默认使用 gRPC 协议.
func (e *etcdDiscovery) Register(ctx context.Context, serviceName, address string) (string, error) {
	return e.RegisterWithProtocol(ctx, serviceName, address, ProtocolGRPC)
}

// RegisterWithProtocol 根据协议注册服务实例.
func (e *etcdDiscovery) RegisterWithProtocol(ctx context.Context, serviceName, address, protocol string) (string, error) {
	return e.RegisterWithHealthEndpoint(ctx, serviceName, address, protocol, nil)
}

// RegisterWithHealthEndpoint 注册服务实例.
// etcd 通过 lease 续约机制实现健康检查，healthEndpoint 参数不使用.
func (e *etcdDiscovery) RegisterWithHealthEndpoint(ctx context.Context, serviceName, address, protocol string, _ *transport.HealthEndpoint) (string, error) {
	if serviceName == "" {
		return "", ErrEmptyName
	}
	if address == "" {
		return "", ErrEmptyAddress
	}

	host, port, err := parseAddress(address)
	if err != nil {
		return "", err
	}
	if host == "0.0.0.0" {
		host = "127.0.0.1"
	}

	serviceMeta := e.config.GetServiceConfig(protocol)
	serviceID := GenerateServiceID(fmt.Sprintf("%s-%s", serviceName, protocol))
	key := fmt.Sprintf("%s%s/%s", etcdServiceKeyPrefix, serviceName, serviceID)

	info := etcdServiceInfo{
		Address:  fmt.Sprintf("%s:%d", host, port),
		Protocol: protocol,
		Version:  serviceMeta.Version,
		Tags:     serviceMeta.Tags,
	}

	value, err := json.Marshal(info)
	if err != nil {
		return "", err
	}

	// 申请 lease
	lease, err := e.client.Grant(ctx, defaultEtcdLeaseTTL)
	if err != nil {
		e.logger.With(logger.Err(err)).Error("[Discovery] etcd申请lease失败")
		return "", ErrRegister
	}

	// 写入服务信息并绑定 lease
	_, err = e.client.Put(ctx, key, string(value), clientv3.WithLease(lease.ID))
	if err != nil {
		e.logger.With(logger.Err(err)).Error("[Discovery] etcd写入服务信息失败")
		return "", ErrRegister
	}

	// 持续续约（使用独立可取消 context，Unregister/Close 时取消以防 goroutine 泄漏）
	keepAliveCtx, keepAliveCancel := context.WithCancel(context.Background())
	keepAliveCh, err := e.client.KeepAlive(keepAliveCtx, lease.ID)
	if err != nil {
		keepAliveCancel()
		e.logger.With(logger.Err(err)).Error("[Discovery] etcd续约失败")
		return "", ErrRegister
	}

	// 消耗续约响应，防止 channel 阻塞
	go func() {
		for range keepAliveCh {
		}
	}()

	e.mu.Lock()
	// 如果同一个 serviceID 已存在（重复注册），先取消旧的 keepalive goroutine
	if oldCancel, ok := e.cancels[serviceID]; ok {
		oldCancel()
	}
	e.leases[serviceID] = lease.ID
	e.cancels[serviceID] = keepAliveCancel
	e.mu.Unlock()

	e.logger.With(
		logger.String("serviceName", serviceName),
		logger.String("serviceID", serviceID),
		logger.String("address", info.Address),
		logger.String("protocol", protocol),
	).Debug("[Discovery] 服务注册成功")

	return serviceID, nil
}

// Unregister 注销服务实例（撤销 lease，自动删除对应 key）.
func (e *etcdDiscovery) Unregister(ctx context.Context, serviceID string) error {
	if serviceID == "" {
		return ErrEmptyServiceID
	}

	e.mu.Lock()
	leaseID, ok := e.leases[serviceID]
	if ok {
		delete(e.leases, serviceID)
	}
	cancelFn, hasCancel := e.cancels[serviceID]
	if hasCancel {
		delete(e.cancels, serviceID)
	}
	e.mu.Unlock()

	// 取消 keepalive goroutine，防止泄漏
	if hasCancel {
		cancelFn()
	}

	if ok {
		if _, err := e.client.Revoke(ctx, leaseID); err != nil {
			e.logger.With(
				logger.String("serviceID", serviceID),
				logger.Err(err),
			).Error("[Discovery] etcd撤销lease失败")
			return ErrUnregister
		}
	}

	e.logger.With(logger.String("serviceID", serviceID)).Debug("[Discovery] 服务注销成功")
	return nil
}

// Discover 通过前缀查询发现服务实例.
func (e *etcdDiscovery) Discover(ctx context.Context, serviceName string) ([]string, error) {
	if serviceName == "" {
		return nil, ErrEmptyName
	}

	prefix := fmt.Sprintf("%s%s/", etcdServiceKeyPrefix, serviceName)
	resp, err := e.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		e.logger.With(
			logger.String("serviceName", serviceName),
			logger.Err(err),
		).Error("[Discovery] etcd服务发现失败")
		return nil, ErrDiscover
	}

	addresses := make([]string, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var info etcdServiceInfo
		if err := json.Unmarshal(kv.Value, &info); err != nil {
			e.logger.With(logger.Err(err)).Warn("[Discovery] 解析服务信息失败，跳过")
			continue
		}
		addresses = append(addresses, info.Address)
		e.logger.With(
			logger.String("serviceName", serviceName),
			logger.String("addr", info.Address),
		).Debug("[Discovery] 发现服务实例")
	}

	if len(addresses) == 0 {
		e.logger.With(logger.String("serviceName", serviceName)).Warn("[Discovery] 未发现任何服务实例")
	}

	return addresses, nil
}

// Close 关闭 etcd 客户端，取消所有 keepalive goroutine.
func (e *etcdDiscovery) Close() error {
	e.mu.Lock()
	for id, cancel := range e.cancels {
		cancel()
		delete(e.cancels, id)
	}
	e.mu.Unlock()
	e.logger.Debug("[Discovery] etcd服务发现连接已关闭")
	return e.client.Close()
}
