package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr error
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: ErrNilConfig,
		},
		{
			name:    "empty type",
			config:  &Config{},
			wantErr: ErrEmptyType,
		},
		{
			name: "custom provider type",
			config: &Config{
				Type: "unknown",
			},
			wantErr: nil,
		},
		{
			name: "valid consul config",
			config: &Config{
				Type: TypeConsul,
				Addr: "localhost:8500",
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

func TestConfig_SetDefaults(t *testing.T) {
	config := &Config{
		Type: TypeConsul,
	}

	config.SetDefaults()

	// HTTP 服务默认配置
	assert.Equal(t, DefaultVersion, config.Services.HTTP.Version)
	assert.Equal(t, ProtocolHTTP, config.Services.HTTP.Protocol)
	assert.Equal(t, []string{"http", "v1"}, config.Services.HTTP.Tags)

	// gRPC 服务默认配置
	assert.Equal(t, DefaultVersion, config.Services.GRPC.Version)
	assert.Equal(t, ProtocolGRPC, config.Services.GRPC.Protocol)
	assert.Equal(t, []string{"grpc", "v1"}, config.Services.GRPC.Tags)
}

func TestConfig_SetDefaults_PreserveExisting(t *testing.T) {
	config := &Config{
		Type: TypeConsul,
		Services: ServiceConfig{
			HTTP: ServiceMetaConfig{
				Version:  "2.0.0",
				Protocol: "https",
				Tags:     []string{"custom"},
			},
		},
	}

	config.SetDefaults()

	// HTTP 自定义配置应该保留
	assert.Equal(t, "2.0.0", config.Services.HTTP.Version)
	assert.Equal(t, "https", config.Services.HTTP.Protocol)
	assert.Equal(t, []string{"custom"}, config.Services.HTTP.Tags)

	// gRPC 未设置，应该使用默认值
	assert.Equal(t, DefaultVersion, config.Services.GRPC.Version)
	assert.Equal(t, ProtocolGRPC, config.Services.GRPC.Protocol)
	assert.Equal(t, []string{"grpc", "v1"}, config.Services.GRPC.Tags)
}

func TestConfig_GetServiceConfig(t *testing.T) {
	config := &Config{
		Type: TypeConsul,
		Services: ServiceConfig{
			HTTP: ServiceMetaConfig{
				Version:  "1.0.0",
				Protocol: ProtocolHTTP,
				Tags:     []string{"http", "v1"},
			},
			GRPC: ServiceMetaConfig{
				Version:  "2.0.0",
				Protocol: ProtocolGRPC,
				Tags:     []string{"grpc", "v2"},
			},
		},
	}

	tests := []struct {
		name     string
		protocol string
		want     ServiceMetaConfig
	}{
		{
			name:     "http protocol",
			protocol: ProtocolHTTP,
			want:     config.Services.HTTP,
		},
		{
			name:     "grpc protocol",
			protocol: ProtocolGRPC,
			want:     config.Services.GRPC,
		},
		{
			name:     "unknown protocol",
			protocol: "websocket",
			want:     ServiceMetaConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.GetServiceConfig(tt.protocol)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConstants(t *testing.T) {
	// 测试类型常量
	assert.Equal(t, "consul", TypeConsul)

	// 测试协议常量
	assert.Equal(t, "http", ProtocolHTTP)
	assert.Equal(t, "grpc", ProtocolGRPC)

	// 测试默认值常量
	assert.Equal(t, "1.0.0", DefaultVersion)
}

func TestServiceMetaConfig(t *testing.T) {
	meta := ServiceMetaConfig{
		Version:  "3.0.0",
		Protocol: "grpc",
		Tags:     []string{"api", "internal"},
	}

	require.Equal(t, "3.0.0", meta.Version)
	require.Equal(t, "grpc", meta.Protocol)
	require.Equal(t, []string{"api", "internal"}, meta.Tags)
}
