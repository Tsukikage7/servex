package discovery

import (
	"github.com/Tsukikage7/servex/v2/errors"
)

var (
	// ErrNilConfig 服务发现配置为空.
	ErrNilConfig = errors.NewWithKind(90001, "discovery.nil_config", "服务发现配置为空", errors.KindInvalidArgument)

	// ErrNilLogger 日志记录器为空.
	ErrNilLogger = errors.NewWithKind(90002, "discovery.nil_logger", "日志记录器为空", errors.KindInvalidArgument)

	// ErrEmptyAddr 服务发现地址为空.
	ErrEmptyAddr = errors.NewWithKind(90003, "discovery.empty_addr", "服务发现地址为空", errors.KindInvalidArgument)

	// ErrEmptyName 服务名称为空.
	ErrEmptyName = errors.NewWithKind(90004, "discovery.empty_name", "服务名称为空", errors.KindInvalidArgument)

	// ErrEmptyAddress 服务地址为空.
	ErrEmptyAddress = errors.NewWithKind(90005, "discovery.empty_address", "服务地址为空", errors.KindInvalidArgument)

	// ErrEmptyServiceID 服务ID为空.
	ErrEmptyServiceID = errors.NewWithKind(90006, "discovery.empty_service_id", "服务ID为空", errors.KindInvalidArgument)

	// ErrEmptyType 服务发现类型为空.
	ErrEmptyType = errors.NewWithKind(90007, "discovery.empty_type", "服务发现类型为空", errors.KindInvalidArgument)

	// ErrUnsupportedType 不支持的服务发现类型.
	ErrUnsupportedType = errors.NewWithKind(90008, "discovery.unsupported_type", "不支持的服务发现类型", errors.KindInvalidArgument)

	// ErrUnsupportedProtocol 不支持的协议类型.
	ErrUnsupportedProtocol = errors.NewWithKind(90009, "discovery.unsupported_protocol", "不支持的协议类型", errors.KindInvalidArgument)

	// ErrInvalidAddress 无效的地址格式.
	ErrInvalidAddress = errors.NewWithKind(90010, "discovery.invalid_address", "无效的地址格式", errors.KindInvalidArgument)

	// ErrInvalidPort 无效的端口号.
	ErrInvalidPort = errors.NewWithKind(90011, "discovery.invalid_port", "无效的端口号", errors.KindInvalidArgument)

	// ErrNotFound 未发现任何服务实例.
	ErrNotFound = errors.NewWithKind(90012, "discovery.not_found", "未发现任何服务实例", errors.KindNotFound)

	// ErrClientCreate 创建客户端失败.
	ErrClientCreate = errors.NewWithKind(90013, "discovery.client_create", "创建客户端失败", errors.KindInternal)

	// ErrRegister 注册服务失败.
	ErrRegister = errors.NewWithKind(90014, "discovery.register", "注册服务失败", errors.KindInternal)

	// ErrUnregister 注销服务失败.
	ErrUnregister = errors.NewWithKind(90015, "discovery.unregister", "注销服务失败", errors.KindInternal)

	// ErrDiscover 发现服务失败.
	ErrDiscover = errors.NewWithKind(90016, "discovery.discover", "发现服务失败", errors.KindInternal)
)
