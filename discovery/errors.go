package discovery

import (
	"net/http"

	"github.com/Tsukikage7/servex/v2/errors"
	"google.golang.org/grpc/codes"
)

var (
	// ErrNilConfig 服务发现配置为空.
	ErrNilConfig = errors.New(90001, "discovery.nil_config", "服务发现配置为空").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)

	// ErrNilLogger 日志记录器为空.
	ErrNilLogger = errors.New(90002, "discovery.nil_logger", "日志记录器为空").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)

	// ErrEmptyAddr 服务发现地址为空.
	ErrEmptyAddr = errors.New(90003, "discovery.empty_addr", "服务发现地址为空").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)

	// ErrEmptyName 服务名称为空.
	ErrEmptyName = errors.New(90004, "discovery.empty_name", "服务名称为空").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)

	// ErrEmptyAddress 服务地址为空.
	ErrEmptyAddress = errors.New(90005, "discovery.empty_address", "服务地址为空").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)

	// ErrEmptyServiceID 服务ID为空.
	ErrEmptyServiceID = errors.New(90006, "discovery.empty_service_id", "服务ID为空").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)

	// ErrEmptyType 服务发现类型为空.
	ErrEmptyType = errors.New(90007, "discovery.empty_type", "服务发现类型为空").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)

	// ErrUnsupportedType 不支持的服务发现类型.
	ErrUnsupportedType = errors.New(90008, "discovery.unsupported_type", "不支持的服务发现类型").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)

	// ErrUnsupportedProtocol 不支持的协议类型.
	ErrUnsupportedProtocol = errors.New(90009, "discovery.unsupported_protocol", "不支持的协议类型").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)

	// ErrInvalidAddress 无效的地址格式.
	ErrInvalidAddress = errors.New(90010, "discovery.invalid_address", "无效的地址格式").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)

	// ErrInvalidPort 无效的端口号.
	ErrInvalidPort = errors.New(90011, "discovery.invalid_port", "无效的端口号").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)

	// ErrNotFound 未发现任何服务实例.
	ErrNotFound = errors.New(90012, "discovery.not_found", "未发现任何服务实例").WithHTTP(http.StatusNotFound).WithGRPC(codes.NotFound)

	// ErrClientCreate 创建客户端失败.
	ErrClientCreate = errors.New(90013, "discovery.client_create", "创建客户端失败").WithHTTP(http.StatusInternalServerError).WithGRPC(codes.Internal)

	// ErrRegister 注册服务失败.
	ErrRegister = errors.New(90014, "discovery.register", "注册服务失败").WithHTTP(http.StatusInternalServerError).WithGRPC(codes.Internal)

	// ErrUnregister 注销服务失败.
	ErrUnregister = errors.New(90015, "discovery.unregister", "注销服务失败").WithHTTP(http.StatusInternalServerError).WithGRPC(codes.Internal)

	// ErrDiscover 发现服务失败.
	ErrDiscover = errors.New(90016, "discovery.discover", "发现服务失败").WithHTTP(http.StatusInternalServerError).WithGRPC(codes.Internal)
)
