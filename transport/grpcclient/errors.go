package grpcclient

import "github.com/Tsukikage7/servex/v2/errors"

var (
	// ErrConnectionFailed 创建连接失败.
	ErrConnectionFailed = errors.New(60101, "transport.grpcclient.connection_failed", "创建连接失败")

	// ErrDiscoveryFailed 服务发现失败.
	ErrDiscoveryFailed = errors.New(60102, "transport.grpcclient.discovery_failed", "服务发现失败")

	// ErrServiceNotFound 未找到服务实例.
	ErrServiceNotFound = errors.New(60103, "transport.grpcclient.service_not_found", "未找到服务实例")

	// ErrMissingServiceName 必须设置 serviceName.
	ErrMissingServiceName = errors.New(60104, "transport.grpcclient.missing_service_name", "必须设置 serviceName")

	// ErrMissingDiscovery 必须设置 discovery.
	ErrMissingDiscovery = errors.New(60105, "transport.grpcclient.missing_discovery", "必须设置 discovery")

	// ErrMissingLogger 必须设置 logger.
	ErrMissingLogger = errors.New(60106, "transport.grpcclient.missing_logger", "必须设置 logger")

	// ErrMissingAddr 配置缺少 addr.
	ErrMissingAddr = errors.New(60107, "transport.grpcclient.missing_addr", "配置缺少 addr")
)
