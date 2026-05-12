package discovery

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/hashicorp/consul/api"

	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/transport"
)

// consulDiscovery 是 Consul 服务发现实现.
type consulDiscovery struct {
	client *api.Client
	config *Config
	logger logger.Logger
}

// newConsulDiscovery 创建 Consul 服务发现实例.
func newConsulDiscovery(config *Config, log logger.Logger) (Discovery, error) {
	// 创建Consul客户端配置
	consulConfig := api.DefaultConfig()
	if config.Addr != "" {
		consulConfig.Address = config.Addr
	}

	// 创建客户端
	client, err := api.NewClient(consulConfig)
	if err != nil {
		log.With(
			logger.String("addr", consulConfig.Address),
			logger.Err(err),
		).Error("[Discovery] 创建 Consul 客户端失败")
		return nil, ErrClientCreate
	}

	return &consulDiscovery{
		client: client,
		config: config,
		logger: log,
	}, nil
}

// Register 注册服务实例，返回服务ID.
func (c *consulDiscovery) Register(ctx context.Context, serviceName, address string) (string, error) {
	if serviceName == "" {
		return "", ErrEmptyName
	}

	if address == "" {
		return "", ErrEmptyAddress
	}

	// 解析地址和端口
	host, port, err := parseAddress(address)
	if err != nil {
		return "", err
	}

	// 如果host是0.0.0.0，转换为127.0.0.1
	if host == "0.0.0.0" {
		host = "127.0.0.1"
	}

	// 生成唯一的服务ID
	serviceID := GenerateServiceID(serviceName)

	// 创建健康检查配置
	healthCheck := c.buildHealthCheck(serviceID, serviceName, host, port)

	// 使用gRPC作为默认配置
	defaultMeta := c.config.GetServiceConfig(ProtocolGRPC)

	registration := &api.AgentServiceRegistration{
		ID:      serviceID,
		Name:    serviceName,
		Address: host,
		Port:    port,
		Tags:    defaultMeta.Tags,
		Meta: map[string]string{
			"version":  defaultMeta.Version,
			"protocol": defaultMeta.Protocol,
		},
		Check: healthCheck,
	}

	// 使用context注册服务
	opts := api.ServiceRegisterOpts{}.WithContext(ctx)
	err = c.client.Agent().ServiceRegisterOpts(registration, opts)
	if err != nil {
		c.logger.With(
			logger.String("serviceName", serviceName),
			logger.String("host", host),
			logger.Int("port", port),
			logger.Err(err),
		).Error("[Discovery] Consul 服务注册失败")
		return "", ErrRegister
	}

	c.logger.With(
		logger.String("serviceName", serviceName),
		logger.String("serviceID", serviceID),
		logger.String("host", host),
		logger.Int("port", port),
	).Debug("[Discovery] 服务注册成功")

	return serviceID, nil
}

// RegisterWithProtocol 根据协议注册服务实例，返回服务ID.
func (c *consulDiscovery) RegisterWithProtocol(ctx context.Context, serviceName, address, protocol string) (string, error) {
	if serviceName == "" {
		return "", ErrEmptyName
	}

	if address == "" {
		return "", ErrEmptyAddress
	}

	// 获取协议特定的元数据配置
	serviceMeta := c.config.GetServiceConfig(protocol)
	if serviceMeta.Protocol == "" {
		return "", ErrUnsupportedProtocol
	}

	// 解析地址和端口
	host, port, err := parseAddress(address)
	if err != nil {
		return "", err
	}

	// 如果host是0.0.0.0，转换为127.0.0.1
	if host == "0.0.0.0" {
		host = "127.0.0.1"
	}

	// 生成协议特定的服务ID
	serviceID := GenerateServiceID(fmt.Sprintf("%s-%s", serviceName, protocol))

	// 创建协议特定的健康检查配置
	healthCheck := c.buildHealthCheckWithProtocol(serviceID, serviceName, host, port, protocol)

	// 使用协议特定的标签
	tags := make([]string, 0, len(serviceMeta.Tags)+1)
	tags = append(tags, serviceMeta.Tags...)
	// 确保协议标签存在
	if !contains(tags, protocol) {
		tags = append(tags, protocol)
	}

	registration := &api.AgentServiceRegistration{
		ID:      serviceID,
		Name:    serviceName,
		Address: host,
		Port:    port,
		Tags:    tags,
		Meta: map[string]string{
			"version":  serviceMeta.Version,
			"protocol": serviceMeta.Protocol,
		},
		Check: healthCheck,
	}

	// 使用context注册服务
	opts := api.ServiceRegisterOpts{}.WithContext(ctx)
	err = c.client.Agent().ServiceRegisterOpts(registration, opts)
	if err != nil {
		c.logger.With(
			logger.String("serviceName", serviceName),
			logger.String("protocol", protocol),
			logger.String("host", host),
			logger.Int("port", port),
			logger.Err(err),
		).Error("[Discovery] Consul 服务注册失败")
		return "", ErrRegister
	}

	c.logger.With(
		logger.String("serviceName", serviceName),
		logger.String("protocol", protocol),
		logger.String("serviceID", serviceID),
		logger.String("host", host),
		logger.Int("port", port),
		logger.String("version", serviceMeta.Version),
		logger.Any("tags", tags),
	).Debug("[Discovery] 服务注册成功")

	return serviceID, nil
}

// RegisterWithHealthEndpoint 使用指定的健康检查端点注册服务.
func (c *consulDiscovery) RegisterWithHealthEndpoint(ctx context.Context, serviceName, address, protocol string, healthEndpoint *transport.HealthEndpoint) (string, error) {
	if serviceName == "" {
		return "", ErrEmptyName
	}

	if address == "" {
		return "", ErrEmptyAddress
	}

	// 获取协议特定的元数据配置
	serviceMeta := c.config.GetServiceConfig(protocol)
	if serviceMeta.Protocol == "" {
		return "", ErrUnsupportedProtocol
	}

	// 解析地址和端口
	host, port, err := parseAddress(address)
	if err != nil {
		return "", err
	}

	// 如果host是0.0.0.0，转换为127.0.0.1
	if host == "0.0.0.0" {
		host = "127.0.0.1"
	}

	// 生成协议特定的服务ID
	serviceID := GenerateServiceID(fmt.Sprintf("%s-%s", serviceName, protocol))

	// 创建健康检查配置
	healthCheck := c.buildHealthCheckFromEndpoint(serviceID, serviceName, host, port, protocol, healthEndpoint)

	// 使用协议特定的标签
	tags := make([]string, 0, len(serviceMeta.Tags)+1)
	tags = append(tags, serviceMeta.Tags...)
	if !contains(tags, protocol) {
		tags = append(tags, protocol)
	}

	registration := &api.AgentServiceRegistration{
		ID:      serviceID,
		Name:    serviceName,
		Address: host,
		Port:    port,
		Tags:    tags,
		Meta: map[string]string{
			"version":  serviceMeta.Version,
			"protocol": serviceMeta.Protocol,
		},
		Check: healthCheck,
	}

	// 使用context注册服务
	opts := api.ServiceRegisterOpts{}.WithContext(ctx)
	err = c.client.Agent().ServiceRegisterOpts(registration, opts)
	if err != nil {
		c.logger.With(
			logger.String("serviceName", serviceName),
			logger.String("protocol", protocol),
			logger.String("host", host),
			logger.Int("port", port),
			logger.Err(err),
		).Error("[Discovery] Consul 服务注册失败")
		return "", ErrRegister
	}

	healthType := "TCP"
	if healthEndpoint != nil {
		healthType = string(healthEndpoint.Type)
	}
	c.logger.With(
		logger.String("serviceName", serviceName),
		logger.String("protocol", protocol),
		logger.String("serviceID", serviceID),
		logger.String("host", host),
		logger.Int("port", port),
		logger.String("healthCheckType", healthType),
		logger.String("version", serviceMeta.Version),
	).Debug("[Discovery] 服务注册成功")

	return serviceID, nil
}

// Unregister 注销服务实例.
func (c *consulDiscovery) Unregister(ctx context.Context, serviceID string) error {
	if serviceID == "" {
		return ErrEmptyServiceID
	}

	// 检查context是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 使用带超时控制的goroutine执行注销操作
	errChan := make(chan error, 1)
	go func() {
		err := c.client.Agent().ServiceDeregister(serviceID)
		errChan <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		if err != nil {
			c.logger.With(
				logger.String("serviceID", serviceID),
				logger.Err(err),
			).Error("[Discovery] Consul 服务注销失败")
			return ErrUnregister
		}
	}

	c.logger.With(logger.String("serviceID", serviceID)).Debug("[Discovery] 服务注销成功")
	return nil
}

// Discover 发现服务实例.
// 默认使用配置中 GRPC 服务的第一个标签进行过滤.
// 如需按其他协议发现，建议使用 DiscoverWithProtocol（待实现）.
func (c *consulDiscovery) Discover(ctx context.Context, serviceName string) ([]string, error) {
	if serviceName == "" {
		return nil, ErrEmptyName
	}

	// 使用配置中的标签过滤服务，不再硬编码 "grpc"
	tag := ""
	if len(c.config.Services.GRPC.Tags) > 0 {
		tag = c.config.Services.GRPC.Tags[0]
	}
	queryOpts := &api.QueryOptions{}
	queryOpts = queryOpts.WithContext(ctx)
	services, _, err := c.client.Health().Service(serviceName, tag, true, queryOpts)
	if err != nil {
		c.logger.With(
			logger.String("serviceName", serviceName),
			logger.Err(err),
		).Error("[Discovery] Consul 服务发现失败")
		return nil, ErrDiscover
	}

	addresses := make([]string, 0, len(services))
	for _, service := range services {
		address := fmt.Sprintf("%s:%d", service.Service.Address, service.Service.Port)
		addresses = append(addresses, address)
		c.logger.With(
			logger.String("serviceName", serviceName),
			logger.String("addr", address),
			logger.Any("tags", service.Service.Tags),
		).Debug("[Discovery] 发现服务实例")
	}

	if len(addresses) == 0 {
		c.logger.With(logger.String("serviceName", serviceName)).Warn("[Discovery] 未发现任何服务实例")
	}

	return addresses, nil
}

// Close 关闭服务发现连接.
func (c *consulDiscovery) Close() error {
	c.logger.Debug("[Discovery] Consul 服务发现连接已关闭")
	return nil
}

// parseAddress 解析地址字符串，返回主机和端口.
func parseAddress(address string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, ErrInvalidAddress
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, ErrInvalidPort
	}

	return host, port, nil
}

// contains 检查字符串切片是否包含指定字符串.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// buildHealthCheck 构建健康检查配置.
func (c *consulDiscovery) buildHealthCheck(serviceID, serviceName, host string, port int) *api.AgentServiceCheck {
	return c.buildHealthCheckWithProtocol(serviceID, serviceName, host, port, "")
}

// buildHealthCheckFromEndpoint 根据健康检查端点构建配置.
func (c *consulDiscovery) buildHealthCheckFromEndpoint(serviceID, serviceName, host string, port int, protocol string, endpoint *transport.HealthEndpoint) *api.AgentServiceCheck {
	check := &api.AgentServiceCheck{
		CheckID:                        fmt.Sprintf("%s-health", serviceID),
		Interval:                       defaultHealthCheckInterval,
		Timeout:                        defaultHealthCheckTimeout,
		DeregisterCriticalServiceAfter: defaultHealthCheckDeregisterAfter,
	}

	// 如果没有指定健康检查端点，使用默认 TCP 检查
	if endpoint == nil {
		check.Name = fmt.Sprintf("%s %s TCP Health Check", serviceName, protocol)
		check.Notes = fmt.Sprintf("TCP端口健康检查 for %s [%s]", serviceName, protocol)
		check.TCP = fmt.Sprintf("%s:%d", host, port)
		return check
	}

	// 解析健康检查地址
	healthHost, healthPort, err := parseAddress(endpoint.Addr)
	if err != nil {
		// 解析失败，使用服务地址
		healthHost = host
		healthPort = port
	}
	if healthHost == "" || healthHost == "0.0.0.0" {
		healthHost = host
	}

	// 根据健康检查端点类型构建配置
	switch endpoint.Type {
	case transport.HealthCheckTypeHTTP:
		path := endpoint.Path
		if path == "" {
			path = defaultHealthCheckHTTPPath
		}
		check.Name = fmt.Sprintf("%s %s HTTP Health Check", serviceName, protocol)
		check.Notes = fmt.Sprintf("HTTP健康检查 for %s [%s]", serviceName, protocol)
		check.HTTP = fmt.Sprintf("http://%s:%d%s", healthHost, healthPort, path)
		check.Method = "GET"
	case transport.HealthCheckTypeGRPC:
		check.Name = fmt.Sprintf("%s %s gRPC Health Check", serviceName, protocol)
		check.Notes = fmt.Sprintf("gRPC健康检查 for %s [%s]", serviceName, protocol)
		check.GRPC = fmt.Sprintf("%s:%d", healthHost, healthPort)
		check.GRPCUseTLS = false
	default: // TCP
		check.Name = fmt.Sprintf("%s %s TCP Health Check", serviceName, protocol)
		check.Notes = fmt.Sprintf("TCP端口健康检查 for %s [%s]", serviceName, protocol)
		check.TCP = fmt.Sprintf("%s:%d", healthHost, healthPort)
	}

	return check
}

// buildHealthCheckWithProtocol 根据协议构建健康检查配置（默认 TCP 检查）.
// 注意：推荐使用 AddServer 方法，可自动检测健康检查类型.
func (c *consulDiscovery) buildHealthCheckWithProtocol(serviceID, serviceName, host string, port int, protocol string) *api.AgentServiceCheck {
	check := &api.AgentServiceCheck{
		CheckID:                        fmt.Sprintf("%s-health", serviceID),
		Interval:                       defaultHealthCheckInterval,
		Timeout:                        defaultHealthCheckTimeout,
		DeregisterCriticalServiceAfter: defaultHealthCheckDeregisterAfter,
	}

	// 默认使用 TCP 端口检查
	if protocol != "" {
		check.Name = fmt.Sprintf("%s %s TCP Health Check", serviceName, protocol)
		check.Notes = fmt.Sprintf("TCP端口健康检查 for %s [%s]", serviceName, protocol)
	} else {
		check.Name = fmt.Sprintf("%s TCP Health Check", serviceName)
		check.Notes = fmt.Sprintf("TCP端口健康检查 for %s", serviceName)
	}
	check.TCP = fmt.Sprintf("%s:%d", host, port)

	return check
}
