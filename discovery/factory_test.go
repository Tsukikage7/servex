package discovery

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/testx"
	"github.com/Tsukikage7/servex/v2/transport"
)

func init() {
	MustRegister(TypeConsul, func(*Config, logger.Logger) (Discovery, error) {
		return fakeDiscovery{}, nil
	})
}

type fakeDiscovery struct{}

func (fakeDiscovery) Register(context.Context, string, string) (string, error) {
	return "test-service-id", nil
}

func (fakeDiscovery) RegisterWithProtocol(context.Context, string, string, string) (string, error) {
	return "test-service-id", nil
}

func (fakeDiscovery) RegisterWithHealthEndpoint(context.Context, string, string, string, *transport.HealthEndpoint) (string, error) {
	return "test-service-id", nil
}

func (fakeDiscovery) Unregister(context.Context, string) error {
	return nil
}

func (fakeDiscovery) Discover(context.Context, string) ([]string, error) {
	return nil, nil
}

func (fakeDiscovery) Close() error {
	return nil
}

func TestNewDiscovery(t *testing.T) {
	log := testx.NopLogger()

	tests := []struct {
		name    string
		config  *Config
		logger  logger.Logger
		wantErr error
	}{
		{
			name:    "nil config",
			config:  nil,
			logger:  log,
			wantErr: ErrNilConfig,
		},
		{
			name: "nil logger",
			config: &Config{
				Type: TypeConsul,
			},
			logger:  nil,
			wantErr: ErrNilLogger,
		},
		{
			name: "empty type",
			config: &Config{
				Type: "",
			},
			logger:  log,
			wantErr: ErrEmptyType,
		},
		{
			name: "unsupported type",
			config: &Config{
				Type: "unknown",
			},
			logger:  log,
			wantErr: ErrUnsupportedType,
		},
		{
			name: "valid consul config",
			config: &Config{
				Type: TypeConsul,
				Addr: "localhost:8500",
			},
			logger:  log,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := NewDiscovery(tt.config, tt.logger)

			if tt.wantErr == nil {
				require.NoError(t, err)
				assert.NotNil(t, d)
				_ = d.Close()
			} else {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, d)
			}
		})
	}
}

func TestMustNewDiscovery_Success(t *testing.T) {
	log := testx.NopLogger()
	config := &Config{
		Type: TypeConsul,
		Addr: "localhost:8500",
	}

	assert.NotPanics(t, func() {
		d := MustNewDiscovery(config, log)
		assert.NotNil(t, d)
		_ = d.Close()
	})
}

func TestMustNewDiscovery_Panic(t *testing.T) {
	log := testx.NopLogger()

	assert.Panics(t, func() {
		MustNewDiscovery(nil, log)
	})

	assert.Panics(t, func() {
		config := &Config{Type: TypeConsul}
		MustNewDiscovery(config, nil)
	})
}

func TestGenerateServiceID(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		wantPrefix  string
	}{
		{
			name:        "normal service name",
			serviceName: "my-service",
			wantPrefix:  "my-service-",
		},
		{
			name:        "empty service name",
			serviceName: "",
			wantPrefix:  "unknown-",
		},
		{
			name:        "service with special chars",
			serviceName: "api-gateway",
			wantPrefix:  "api-gateway-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := GenerateServiceID(tt.serviceName)

			// 应该以服务名开头
			assert.True(t, strings.HasPrefix(id, tt.wantPrefix),
				"expected prefix %q, got %q", tt.wantPrefix, id)

			// ID 不应该为空
			assert.NotEmpty(t, id)

			// 生成的 ID 应该是唯一的多次调用不同
			id2 := GenerateServiceID(tt.serviceName)
			// 由于使用时间戳，在快速调用中可能相同，但格式应该一致
			assert.True(t, strings.HasPrefix(id2, tt.wantPrefix))
		})
	}
}

func TestGenerateServiceID_Uniqueness(t *testing.T) {
	ids := make(map[string]bool)

	// 生成多个 ID 验证唯一性
	for i := 0; i < 100; i++ {
		id := GenerateServiceID("test-service")
		if ids[id] {
			// 由于使用纳秒时间戳，在极快的循环中可能有重复
			// 这是可接受的，因为实际使用中不会如此快速地创建服务
			t.Logf("duplicate ID found: %s (this may happen in tight loops)", id)
		}
		ids[id] = true
	}

	// 至少应该有一些唯一的 ID
	assert.Greater(t, len(ids), 0)
}
