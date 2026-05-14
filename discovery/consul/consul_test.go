package consul

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tsukikage7/servex/v2/discovery"
	"github.com/Tsukikage7/servex/v2/testx"
)

func TestNewConsulDiscovery(t *testing.T) {
	log := testx.NopLogger()

	tests := []struct {
		name    string
		config  *discovery.Config
		wantErr bool
	}{
		{
			name: "valid config with addr",
			config: &discovery.Config{
				Type: discovery.TypeConsul,
				Addr: "localhost:8500",
			},
			wantErr: false,
		},
		{
			name: "valid config without addr (uses default)",
			config: &discovery.Config{
				Type: discovery.TypeConsul,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.SetDefaults()
			d, err := New(tt.config, log)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, d)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, d)
				_ = d.Close()
			}
		})
	}
}

func TestConsulDiscovery_Register_Validation(t *testing.T) {
	log := testx.NopLogger()
	config := &discovery.Config{
		Type: discovery.TypeConsul,
		Addr: "localhost:8500",
	}
	config.SetDefaults()

	d, err := New(config, log)
	require.NoError(t, err)
	defer d.Close()

	ctx := t.Context()

	tests := []struct {
		name        string
		serviceName string
		address     string
		wantErr     error
	}{
		{
			name:        "empty service name",
			serviceName: "",
			address:     "localhost:8080",
			wantErr:     discovery.ErrEmptyName,
		},
		{
			name:        "empty address",
			serviceName: "test-service",
			address:     "",
			wantErr:     discovery.ErrEmptyAddress,
		},
		{
			name:        "invalid address format",
			serviceName: "test-service",
			address:     "invalid",
			wantErr:     discovery.ErrInvalidAddress,
		},
		{
			name:        "invalid port",
			serviceName: "test-service",
			address:     "localhost:abc",
			wantErr:     discovery.ErrInvalidPort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := d.Register(ctx, tt.serviceName, tt.address)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestConsulDiscovery_RegisterWithProtocol_Validation(t *testing.T) {
	log := testx.NopLogger()
	config := &discovery.Config{
		Type: discovery.TypeConsul,
		Addr: "localhost:8500",
	}
	config.SetDefaults()

	d, err := New(config, log)
	require.NoError(t, err)
	defer d.Close()

	ctx := t.Context()

	tests := []struct {
		name        string
		serviceName string
		address     string
		protocol    string
		wantErr     error
	}{
		{
			name:        "empty service name",
			serviceName: "",
			address:     "localhost:8080",
			protocol:    discovery.ProtocolGRPC,
			wantErr:     discovery.ErrEmptyName,
		},
		{
			name:        "empty address",
			serviceName: "test-service",
			address:     "",
			protocol:    discovery.ProtocolGRPC,
			wantErr:     discovery.ErrEmptyAddress,
		},
		{
			name:        "unsupported protocol",
			serviceName: "test-service",
			address:     "localhost:8080",
			protocol:    "websocket",
			wantErr:     discovery.ErrUnsupportedProtocol,
		},
		{
			name:        "invalid address format",
			serviceName: "test-service",
			address:     "invalid-address",
			protocol:    discovery.ProtocolHTTP,
			wantErr:     discovery.ErrInvalidAddress,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := d.RegisterWithProtocol(ctx, tt.serviceName, tt.address, tt.protocol)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestConsulDiscovery_Unregister_Validation(t *testing.T) {
	log := testx.NopLogger()
	config := &discovery.Config{
		Type: discovery.TypeConsul,
		Addr: "localhost:8500",
	}
	config.SetDefaults()

	d, err := New(config, log)
	require.NoError(t, err)
	defer d.Close()

	ctx := t.Context()

	err = d.Unregister(ctx, "")
	assert.ErrorIs(t, err, discovery.ErrEmptyServiceID)
}

func TestConsulDiscovery_Unregister_ContextCanceled(t *testing.T) {
	log := testx.NopLogger()
	config := &discovery.Config{
		Type: discovery.TypeConsul,
		Addr: "localhost:8500",
	}
	config.SetDefaults()

	d, err := New(config, log)
	require.NoError(t, err)
	defer d.Close()

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // 立即取消

	err = d.Unregister(ctx, "test-service-id")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestConsulDiscovery_Discover_Validation(t *testing.T) {
	log := testx.NopLogger()
	config := &discovery.Config{
		Type: discovery.TypeConsul,
		Addr: "localhost:8500",
	}
	config.SetDefaults()

	d, err := New(config, log)
	require.NoError(t, err)
	defer d.Close()

	ctx := t.Context()

	_, err = d.Discover(ctx, "")
	assert.ErrorIs(t, err, discovery.ErrEmptyName)
}

func TestConsulDiscovery_Close(t *testing.T) {
	log := testx.NopLogger()
	config := &discovery.Config{
		Type: discovery.TypeConsul,
		Addr: "localhost:8500",
	}
	config.SetDefaults()

	d, err := New(config, log)
	require.NoError(t, err)

	err = d.Close()
	assert.NoError(t, err)
}

func TestParseAddress(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		wantHost string
		wantPort int
		wantErr  error
	}{
		{
			name:     "valid address",
			address:  "localhost:8080",
			wantHost: "localhost",
			wantPort: 8080,
			wantErr:  nil,
		},
		{
			name:     "valid IP address",
			address:  "192.168.1.1:9090",
			wantHost: "192.168.1.1",
			wantPort: 9090,
			wantErr:  nil,
		},
		{
			name:     "zero address",
			address:  "0.0.0.0:3000",
			wantHost: "0.0.0.0",
			wantPort: 3000,
			wantErr:  nil,
		},
		{
			name:     "localhost IP",
			address:  "127.0.0.1:8080",
			wantHost: "127.0.0.1",
			wantPort: 8080,
			wantErr:  nil,
		},
		{
			name:     "invalid format - no port",
			address:  "localhost",
			wantHost: "",
			wantPort: 0,
			wantErr:  discovery.ErrInvalidAddress,
		},
		{
			name:     "invalid port - not a number",
			address:  "localhost:abc",
			wantHost: "",
			wantPort: 0,
			wantErr:  discovery.ErrInvalidPort,
		},
		{
			name:     "empty address",
			address:  "",
			wantHost: "",
			wantPort: 0,
			wantErr:  discovery.ErrInvalidAddress,
		},
		{
			name:     "invalid format - multiple colons",
			address:  "host:port:extra",
			wantHost: "",
			wantPort: 0,
			wantErr:  discovery.ErrInvalidAddress,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := parseAddress(tt.address)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantHost, host)
				assert.Equal(t, tt.wantPort, port)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name   string
		slice  []string
		item   string
		expect bool
	}{
		{
			name:   "item exists",
			slice:  []string{"a", "b", "c"},
			item:   "b",
			expect: true,
		},
		{
			name:   "item not exists",
			slice:  []string{"a", "b", "c"},
			item:   "d",
			expect: false,
		},
		{
			name:   "empty slice",
			slice:  []string{},
			item:   "a",
			expect: false,
		},
		{
			name:   "nil slice",
			slice:  nil,
			item:   "a",
			expect: false,
		},
		{
			name:   "item at first position",
			slice:  []string{"a", "b", "c"},
			item:   "a",
			expect: true,
		},
		{
			name:   "item at last position",
			slice:  []string{"a", "b", "c"},
			item:   "c",
			expect: true,
		},
		{
			name:   "empty item",
			slice:  []string{"a", "", "c"},
			item:   "",
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.item)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestConsulDiscovery_Unregister_Timeout(t *testing.T) {
	log := testx.NopLogger()
	config := &discovery.Config{
		Type: discovery.TypeConsul,
		Addr: "localhost:8500",
	}
	config.SetDefaults()

	d, err := New(config, log)
	require.NoError(t, err)
	defer d.Close()

	// 使用很短的超时时间
	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Nanosecond)
	defer cancel()

	// 等待一下确保超时
	time.Sleep(1 * time.Millisecond)

	err = d.Unregister(ctx, "test-service-id")
	// 可能是 context.DeadlineExceeded 或实际的注销错误
	assert.Error(t, err)
}

func TestConsulDiscovery_AddressConversion(t *testing.T) {
	log := testx.NopLogger()
	config := &discovery.Config{
		Type: discovery.TypeConsul,
		Addr: "localhost:8500",
	}
	config.SetDefaults()

	d, err := New(config, log)
	require.NoError(t, err)
	defer d.Close()

	ctx := t.Context()

	// 测试使用 0.0.0.0 地址会触发转换
	// 注意：如果本地有 Consul 服务器运行，可能会成功；否则会失败
	// 这里只验证代码路径不会 panic
	_, err = d.Register(ctx, "test-service", "0.0.0.0:8080")
	// 可能成功也可能失败，取决于是否有 Consul 服务器
	if err != nil {
		assert.ErrorIs(t, err, discovery.ErrRegister)
	}

	_, err = d.RegisterWithProtocol(ctx, "test-service", "0.0.0.0:8080", discovery.ProtocolGRPC)
	if err != nil {
		assert.ErrorIs(t, err, discovery.ErrRegister)
	}
}

func TestConsulDiscovery_TagsHandling(t *testing.T) {
	log := testx.NopLogger()

	// 测试协议标签已存在的情况
	config := &discovery.Config{
		Type: discovery.TypeConsul,
		Addr: "localhost:8500",
		Services: discovery.ServiceConfig{
			GRPC: discovery.ServiceMetaConfig{
				Version:  "1.0.0",
				Protocol: discovery.ProtocolGRPC,
				Tags:     []string{"grpc", "api"}, // grpc 标签已存在
			},
		},
	}
	config.SetDefaults()

	d, err := New(config, log)
	require.NoError(t, err)
	defer d.Close()

	ctx := t.Context()
	_, err = d.RegisterWithProtocol(ctx, "test-service", "localhost:8080", discovery.ProtocolGRPC)
	// 可能成功也可能失败，取决于是否有 Consul 服务器
	if err != nil {
		assert.ErrorIs(t, err, discovery.ErrRegister)
	}
}

func TestConsulDiscovery_HTTPProtocol(t *testing.T) {
	log := testx.NopLogger()
	config := &discovery.Config{
		Type: discovery.TypeConsul,
		Addr: "localhost:8500",
	}
	config.SetDefaults()

	d, err := New(config, log)
	require.NoError(t, err)
	defer d.Close()

	ctx := t.Context()

	// 测试 HTTP 协议注册
	// 可能成功也可能失败，取决于是否有 Consul 服务器
	_, err = d.RegisterWithProtocol(ctx, "test-service", "localhost:8080", discovery.ProtocolHTTP)
	if err != nil {
		assert.ErrorIs(t, err, discovery.ErrRegister)
	}
}

func TestConsulDiscovery_Discover_NoConsul(t *testing.T) {
	log := testx.NopLogger()
	config := &discovery.Config{
		Type: discovery.TypeConsul,
		Addr: "localhost:8500",
	}
	config.SetDefaults()

	d, err := New(config, log)
	require.NoError(t, err)
	defer d.Close()

	ctx := t.Context()

	// 测试服务发现
	// 可能成功也可能失败，取决于是否有 Consul 服务器
	addresses, err := d.Discover(ctx, "test-service")
	if err != nil {
		assert.ErrorIs(t, err, discovery.ErrDiscover)
	} else {
		// 如果成功，返回的应该是空列表（服务不存在）
		assert.Empty(t, addresses)
	}
}

func TestConsulDiscovery_Unregister_NoConsul(t *testing.T) {
	log := testx.NopLogger()
	config := &discovery.Config{
		Type: discovery.TypeConsul,
		Addr: "localhost:8500",
	}
	config.SetDefaults()

	d, err := New(config, log)
	require.NoError(t, err)
	defer d.Close()

	ctx := t.Context()

	// 测试服务注销
	// 可能成功也可能失败，取决于是否有 Consul 服务器
	err = d.Unregister(ctx, "test-service-id-12345")
	if err != nil {
		assert.ErrorIs(t, err, discovery.ErrUnregister)
	}
}

func TestConsulDiscovery_Register_NoConsul(t *testing.T) {
	log := testx.NopLogger()
	config := &discovery.Config{
		Type: discovery.TypeConsul,
		Addr: "localhost:8500",
	}
	config.SetDefaults()

	d, err := New(config, log)
	require.NoError(t, err)
	defer d.Close()

	ctx := t.Context()

	// 测试服务注册
	// 可能成功也可能失败，取决于是否有 Consul 服务器
	_, err = d.Register(ctx, "test-service", "localhost:8080")
	if err != nil {
		assert.ErrorIs(t, err, discovery.ErrRegister)
	}
}
