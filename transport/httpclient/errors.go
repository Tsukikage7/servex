package httpclient

import "github.com/Tsukikage7/servex/v2/errors"

var (
	// ErrRequestFailed 请求创建失败.
	ErrRequestFailed = errors.New(60001, "transport.httpclient.request_failed", "请求创建失败")

	// ErrDiscoveryFailed 服务发现失败.
	ErrDiscoveryFailed = errors.New(60002, "transport.httpclient.discovery_failed", "服务发现失败")

	// ErrServiceNotFound 未找到服务实例.
	ErrServiceNotFound = errors.New(60003, "transport.httpclient.service_not_found", "未找到服务实例")

	// ErrMarshalBody 请求体序列化失败.
	ErrMarshalBody = errors.New(60004, "transport.httpclient.marshal_body", "请求体序列化失败")

	// ErrMissingServiceName 必须设置 serviceName.
	ErrMissingServiceName = errors.New(60005, "transport.httpclient.missing_service_name", "必须设置 serviceName")

	// ErrMissingDiscovery 必须设置 discovery.
	ErrMissingDiscovery = errors.New(60006, "transport.httpclient.missing_discovery", "必须设置 discovery")

	// ErrMissingLogger 必须设置 logger.
	ErrMissingLogger = errors.New(60007, "transport.httpclient.missing_logger", "必须设置 logger")
)
