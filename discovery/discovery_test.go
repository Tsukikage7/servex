package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrors(t *testing.T) {
	// 测试所有错误常量都已正确定义
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{"ErrNilConfig", ErrNilConfig, "服务发现配置为空"},
		{"ErrNilLogger", ErrNilLogger, "日志记录器为空"},
		{"ErrEmptyAddr", ErrEmptyAddr, "服务发现地址为空"},
		{"ErrEmptyName", ErrEmptyName, "服务名称为空"},
		{"ErrEmptyAddress", ErrEmptyAddress, "服务地址为空"},
		{"ErrEmptyServiceID", ErrEmptyServiceID, "服务ID为空"},
		{"ErrEmptyType", ErrEmptyType, "服务发现类型为空"},
		{"ErrUnsupportedType", ErrUnsupportedType, "不支持的服务发现类型"},
		{"ErrUnsupportedProtocol", ErrUnsupportedProtocol, "不支持的协议类型"},
		{"ErrInvalidAddress", ErrInvalidAddress, "无效的地址格式"},
		{"ErrInvalidPort", ErrInvalidPort, "无效的端口号"},
		{"ErrNotFound", ErrNotFound, "未发现任何服务实例"},
		{"ErrClientCreate", ErrClientCreate, "创建客户端失败"},
		{"ErrRegister", ErrRegister, "注册服务失败"},
		{"ErrUnregister", ErrUnregister, "注销服务失败"},
		{"ErrDiscover", ErrDiscover, "发现服务失败"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.err)
			assert.Contains(t, tt.err.Error(), tt.msg)
		})
	}
}
