package discovery

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/transport"
)

// mockDiscovery 模拟服务发现
type mockDiscovery struct {
	mock.Mock
	mu sync.Mutex
}

func (m *mockDiscovery) Register(ctx context.Context, serviceName, address string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	args := m.Called(ctx, serviceName, address)
	return args.String(0), args.Error(1)
}

func (m *mockDiscovery) RegisterWithProtocol(ctx context.Context, serviceName, address, protocol string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	args := m.Called(ctx, serviceName, address, protocol)
	return args.String(0), args.Error(1)
}

func (m *mockDiscovery) RegisterWithHealthEndpoint(ctx context.Context, serviceName, address, protocol string, healthEndpoint *transport.HealthEndpoint) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	args := m.Called(ctx, serviceName, address, protocol, healthEndpoint)
	return args.String(0), args.Error(1)
}

func (m *mockDiscovery) Unregister(ctx context.Context, serviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	args := m.Called(ctx, serviceID)
	return args.Error(0)
}

func (m *mockDiscovery) Discover(ctx context.Context, serviceName string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	args := m.Called(ctx, serviceName)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockDiscovery) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	args := m.Called()
	return args.Error(0)
}

// registryMockLogger 模拟日志
type registryMockLogger struct{}

func (m *registryMockLogger) Debug(args ...any)                             {}
func (m *registryMockLogger) Debugf(format string, args ...any)             {}
func (m *registryMockLogger) Info(args ...any)                              {}
func (m *registryMockLogger) Infof(format string, args ...any)              {}
func (m *registryMockLogger) Warn(args ...any)                              {}
func (m *registryMockLogger) Warnf(format string, args ...any)              {}
func (m *registryMockLogger) Error(args ...any)                             {}
func (m *registryMockLogger) Errorf(format string, args ...any)             {}
func (m *registryMockLogger) Fatal(args ...any)                             {}
func (m *registryMockLogger) Fatalf(format string, args ...any)             {}
func (m *registryMockLogger) Panic(args ...any)                             {}
func (m *registryMockLogger) Panicf(format string, args ...any)             {}
func (m *registryMockLogger) With(fields ...logger.Field) logger.Logger     { return m }
func (m *registryMockLogger) WithContext(ctx context.Context) logger.Logger { return m }
func (m *registryMockLogger) Sync() error                                   { return nil }
func (m *registryMockLogger) Close() error                                  { return nil }

func TestServiceRegistry_AddService(t *testing.T) {
	disc := &mockDiscovery{}
	log := &registryMockLogger{}
	registry := NewServiceRegistry(disc, log)

	registry.AddService("test-service", "localhost:8080", ProtocolHTTP)
	registry.AddGRPC("grpc-service", "localhost:9090")
	registry.AddHTTP("http-service", "localhost:8081")

	assert.Len(t, registry.services, 3)
	assert.Equal(t, "test-service", registry.services[0].Name)
	assert.Equal(t, ProtocolHTTP, registry.services[0].Protocol)
	assert.Equal(t, ProtocolGRPC, registry.services[1].Protocol)
	assert.Equal(t, ProtocolHTTP, registry.services[2].Protocol)
}

func TestServiceRegistry_RegisterAll_Success(t *testing.T) {
	disc := &mockDiscovery{}
	log := &registryMockLogger{}
	registry := NewServiceRegistry(disc, log)

	ctx := t.Context()

	disc.On("RegisterWithProtocol", ctx, "grpc-service", "localhost:9090", ProtocolGRPC).
		Return("grpc-id-123", nil)
	disc.On("RegisterWithProtocol", ctx, "http-service", "localhost:8080", ProtocolHTTP).
		Return("http-id-456", nil)

	registry.AddGRPC("grpc-service", "localhost:9090")
	registry.AddHTTP("http-service", "localhost:8080")

	err := registry.RegisterAll(ctx)

	assert.NoError(t, err)
	assert.Len(t, registry.serviceIDs, 2)
	assert.Contains(t, registry.serviceIDs, "grpc-id-123")
	assert.Contains(t, registry.serviceIDs, "http-id-456")
	disc.AssertExpectations(t)
}

func TestServiceRegistry_RegisterAll_Failure_Rollback(t *testing.T) {
	disc := &mockDiscovery{}
	log := &registryMockLogger{}
	registry := NewServiceRegistry(disc, log)

	ctx := t.Context()

	// 第一个服务注册成功
	disc.On("RegisterWithProtocol", ctx, "grpc-service", "localhost:9090", ProtocolGRPC).
		Return("grpc-id-123", nil)
	// 第二个服务注册失败
	disc.On("RegisterWithProtocol", ctx, "http-service", "localhost:8080", ProtocolHTTP).
		Return("", errors.New("register failed"))
	// 回滚：注销第一个服务
	disc.On("Unregister", ctx, "grpc-id-123").Return(nil)

	registry.AddGRPC("grpc-service", "localhost:9090")
	registry.AddHTTP("http-service", "localhost:8080")

	err := registry.RegisterAll(ctx)

	assert.Error(t, err)
	assert.Len(t, registry.serviceIDs, 0) // 回滚后应该清空
	disc.AssertExpectations(t)
}

func TestServiceRegistry_UnregisterAll(t *testing.T) {
	disc := &mockDiscovery{}
	log := &registryMockLogger{}
	registry := NewServiceRegistry(disc, log)

	ctx := t.Context()

	disc.On("RegisterWithProtocol", ctx, "grpc-service", "localhost:9090", ProtocolGRPC).
		Return("grpc-id-123", nil)
	disc.On("Unregister", ctx, "grpc-id-123").Return(nil)

	registry.AddGRPC("grpc-service", "localhost:9090")
	_ = registry.RegisterAll(ctx)

	err := registry.UnregisterAll(ctx)

	assert.NoError(t, err)
	assert.Len(t, registry.serviceIDs, 0)
	disc.AssertExpectations(t)
}

func TestServiceRegistry_Hooks(t *testing.T) {
	disc := &mockDiscovery{}
	log := &registryMockLogger{}
	registry := NewServiceRegistry(disc, log)

	ctx := t.Context()

	disc.On("RegisterWithProtocol", ctx, "test-service", "localhost:8080", ProtocolGRPC).
		Return("test-id", nil)
	disc.On("Unregister", ctx, "test-id").Return(nil)

	registry.AddGRPC("test-service", "localhost:8080")

	// 测试 AfterStartHook
	afterStart := registry.AfterStartHook()
	err := afterStart(ctx)
	assert.NoError(t, err)

	// 测试 BeforeStopHook
	beforeStop := registry.BeforeStopHook()
	err = beforeStop(ctx)
	assert.NoError(t, err)

	disc.AssertExpectations(t)
}

func TestServiceRegistry_ChainedCalls(t *testing.T) {
	disc := &mockDiscovery{}
	log := &registryMockLogger{}

	registry := NewServiceRegistry(disc, log).
		AddGRPC("grpc-service", "localhost:9090").
		AddHTTP("http-service", "localhost:8080").
		AddService("custom-service", "localhost:8081", "custom")

	assert.Len(t, registry.services, 3)
}
