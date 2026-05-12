package discovery

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"

	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/transport"
)

const (
	// defaultNacosGroupName 默认 Nacos 分组名.
	defaultNacosGroupName = "DEFAULT_GROUP"
)

// nacosDiscovery 基于 Nacos 的服务发现实现.
type nacosDiscovery struct {
	client    naming_client.INamingClient
	config    *Config
	logger    logger.Logger
	groupName string
	mu        sync.Mutex
	instances map[string]nacosInstanceInfo // serviceID -> instanceInfo
}

// nacosInstanceInfo 已注册的 Nacos 实例信息.
type nacosInstanceInfo struct {
	serviceName string
	ip          string
	port        uint64
}

// 编译期接口合规检查.
var _ Discovery = (*nacosDiscovery)(nil)

// newNacosDiscovery 创建 Nacos 服务发现实例.
func newNacosDiscovery(config *Config, log logger.Logger) (Discovery, error) {
	endpoints := config.NacosEndpoints
	if len(endpoints) == 0 {
		endpoints = []string{"127.0.0.1:8848"}
	}

	serverConfigs := make([]constant.ServerConfig, 0, len(endpoints))
	for _, ep := range endpoints {
		host, portStr, err := net.SplitHostPort(ep)
		if err != nil {
			log.With(logger.Err(err)).Error("[Discovery] 解析 Nacos 端点失败")
			return nil, ErrClientCreate
		}
		port, err := strconv.ParseUint(portStr, 10, 64)
		if err != nil {
			log.With(logger.Err(err)).Error("[Discovery] 解析 Nacos 端口失败")
			return nil, ErrClientCreate
		}
		serverConfigs = append(serverConfigs, constant.ServerConfig{
			IpAddr: host,
			Port:   port,
		})
	}

	clientConfig := constant.NewClientConfig(
		constant.WithNamespaceId(config.NacosNamespaceID),
		constant.WithNotLoadCacheAtStart(true),
	)

	namingClient, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  clientConfig,
			ServerConfigs: serverConfigs,
		},
	)
	if err != nil {
		log.With(logger.Err(err)).Error("[Discovery] 创建 Nacos 客户端失败")
		return nil, ErrClientCreate
	}

	groupName := config.NacosGroupName
	if groupName == "" {
		groupName = defaultNacosGroupName
	}

	return &nacosDiscovery{
		client:    namingClient,
		config:    config,
		logger:    log,
		groupName: groupName,
		instances: make(map[string]nacosInstanceInfo),
	}, nil
}

// Register 注册服务实例，默认使用 gRPC 协议.
func (n *nacosDiscovery) Register(ctx context.Context, serviceName, address string) (string, error) {
	return n.RegisterWithProtocol(ctx, serviceName, address, ProtocolGRPC)
}

// RegisterWithProtocol 根据协议注册服务实例.
func (n *nacosDiscovery) RegisterWithProtocol(ctx context.Context, serviceName, address, protocol string) (string, error) {
	return n.RegisterWithHealthEndpoint(ctx, serviceName, address, protocol, nil)
}

// RegisterWithHealthEndpoint 注册服务实例.
// Nacos 自带健康检查机制，healthEndpoint 参数不使用.
func (n *nacosDiscovery) RegisterWithHealthEndpoint(ctx context.Context, serviceName, address, protocol string, _ *transport.HealthEndpoint) (string, error) {
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

	serviceMeta := n.config.GetServiceConfig(protocol)
	serviceID := GenerateServiceID(fmt.Sprintf("%s-%s", serviceName, protocol))

	metadata := map[string]string{
		"protocol": protocol,
		"version":  serviceMeta.Version,
	}

	success, err := n.client.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          host,
		Port:        uint64(port),
		ServiceName: serviceName,
		GroupName:   n.groupName,
		Weight:      1,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		Metadata:    metadata,
	})
	if err != nil || !success {
		n.logger.With(logger.Err(err)).Error("[Discovery] Nacos 注册服务失败")
		return "", ErrRegister
	}

	n.mu.Lock()
	n.instances[serviceID] = nacosInstanceInfo{
		serviceName: serviceName,
		ip:          host,
		port:        uint64(port),
	}
	n.mu.Unlock()

	n.logger.With(
		logger.String("serviceName", serviceName),
		logger.String("serviceID", serviceID),
		logger.String("address", fmt.Sprintf("%s:%d", host, port)),
		logger.String("protocol", protocol),
	).Debug("[Discovery] 服务注册成功")

	return serviceID, nil
}

// Unregister 注销服务实例.
func (n *nacosDiscovery) Unregister(ctx context.Context, serviceID string) error {
	if serviceID == "" {
		return ErrEmptyServiceID
	}

	// 检查 ctx 是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	n.mu.Lock()
	info, ok := n.instances[serviceID]
	if ok {
		delete(n.instances, serviceID)
	}
	n.mu.Unlock()

	if !ok {
		return nil
	}

	// 通过 goroutine + select 传递 ctx 超时控制
	type result struct {
		success bool
		err     error
	}
	ch := make(chan result, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- result{false, fmt.Errorf("deregister panic: %v", r)}
			}
		}()
		s, e := n.client.DeregisterInstance(vo.DeregisterInstanceParam{
			Ip:          info.ip,
			Port:        info.port,
			ServiceName: info.serviceName,
			GroupName:   n.groupName,
			Ephemeral:   true,
		})
		ch <- result{s, e}
	}()

	var success bool
	var err error
	select {
	case <-ctx.Done():
		return ctx.Err()
	case r := <-ch:
		success, err = r.success, r.err
	}
	if err != nil || !success {
		n.logger.With(
			logger.String("serviceID", serviceID),
			logger.Err(err),
		).Error("[Discovery] Nacos 注销服务失败")
		return ErrUnregister
	}

	n.logger.With(logger.String("serviceID", serviceID)).Debug("[Discovery] 服务注销成功")
	return nil
}

// Discover 发现服务实例.
func (n *nacosDiscovery) Discover(ctx context.Context, serviceName string) ([]string, error) {
	if serviceName == "" {
		return nil, ErrEmptyName
	}

	instances, err := n.client.SelectInstances(vo.SelectInstancesParam{
		ServiceName: serviceName,
		GroupName:   n.groupName,
		HealthyOnly: true,
	})
	if err != nil {
		n.logger.With(
			logger.String("serviceName", serviceName),
			logger.Err(err),
		).Error("[Discovery] Nacos 服务发现失败")
		return nil, ErrDiscover
	}

	addresses := make([]string, 0, len(instances))
	for _, inst := range instances {
		addr := fmt.Sprintf("%s:%d", inst.Ip, inst.Port)
		addresses = append(addresses, addr)
		n.logger.With(
			logger.String("serviceName", serviceName),
			logger.String("addr", addr),
		).Debug("[Discovery] 发现服务实例")
	}

	if len(addresses) == 0 {
		n.logger.With(logger.String("serviceName", serviceName)).Warn("[Discovery] 未发现任何服务实例")
	}

	return addresses, nil
}

// Close 关闭 Nacos 客户端.
func (n *nacosDiscovery) Close() error {
	n.logger.Debug("[Discovery] Nacos 服务发现连接已关闭")
	n.client.CloseClient()
	return nil
}
