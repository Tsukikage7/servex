package discovery

import (
	"errors"
	"fmt"
	"testing"

	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"

	"github.com/Tsukikage7/servex/v2/testx"
)

// mockNamingClient 模拟 Nacos naming client.
type mockNamingClient struct {
	registerFn   func(param vo.RegisterInstanceParam) (bool, error)
	deregisterFn func(param vo.DeregisterInstanceParam) (bool, error)
	selectFn     func(param vo.SelectInstancesParam) ([]model.Instance, error)
}

func (m *mockNamingClient) RegisterInstance(param vo.RegisterInstanceParam) (bool, error) {
	if m.registerFn != nil {
		return m.registerFn(param)
	}
	return true, nil
}

func (m *mockNamingClient) BatchRegisterInstance(param vo.BatchRegisterInstanceParam) (bool, error) {
	return true, nil
}

func (m *mockNamingClient) DeregisterInstance(param vo.DeregisterInstanceParam) (bool, error) {
	if m.deregisterFn != nil {
		return m.deregisterFn(param)
	}
	return true, nil
}

func (m *mockNamingClient) UpdateInstance(param vo.UpdateInstanceParam) (bool, error) {
	return true, nil
}

func (m *mockNamingClient) GetService(param vo.GetServiceParam) (model.Service, error) {
	return model.Service{}, nil
}

func (m *mockNamingClient) SelectAllInstances(param vo.SelectAllInstancesParam) ([]model.Instance, error) {
	return nil, nil
}

func (m *mockNamingClient) SelectInstances(param vo.SelectInstancesParam) ([]model.Instance, error) {
	if m.selectFn != nil {
		return m.selectFn(param)
	}
	return nil, nil
}

func (m *mockNamingClient) SelectOneHealthyInstance(param vo.SelectOneHealthInstanceParam) (*model.Instance, error) {
	return nil, nil
}

func (m *mockNamingClient) Subscribe(param *vo.SubscribeParam) error {
	return nil
}

func (m *mockNamingClient) Unsubscribe(param *vo.SubscribeParam) error {
	return nil
}

func (m *mockNamingClient) GetAllServicesInfo(param vo.GetAllServiceInfoParam) (model.ServiceList, error) {
	return model.ServiceList{}, nil
}

func (m *mockNamingClient) ServerHealthy() bool {
	return true
}

func (m *mockNamingClient) CloseClient() {}

func newMockNacosDiscovery(mock *mockNamingClient) *nacosDiscovery {
	cfg := &Config{
		Type: TypeNacos,
	}
	cfg.SetDefaults()
	return &nacosDiscovery{
		client:    mock,
		config:    cfg,
		logger:    testx.NopLogger(),
		groupName: defaultNacosGroupName,
		instances: make(map[string]nacosInstanceInfo),
	}
}

func TestNacosDiscovery_Register(t *testing.T) {
	mock := &mockNamingClient{}
	d := newMockNacosDiscovery(mock)

	ctx := t.Context()

	serviceID, err := d.Register(ctx, "test-service", "127.0.0.1:9090")
	if err != nil {
		t.Fatalf("注册服务失败: %v", err)
	}
	if serviceID == "" {
		t.Fatal("serviceID 不应为空")
	}
}

func TestNacosDiscovery_RegisterEmptyName(t *testing.T) {
	mock := &mockNamingClient{}
	d := newMockNacosDiscovery(mock)

	_, err := d.Register(t.Context(), "", "127.0.0.1:9090")
	if !errors.Is(err, ErrEmptyName) {
		t.Errorf("期望 ErrEmptyName，得到 %v", err)
	}
}

func TestNacosDiscovery_RegisterEmptyAddress(t *testing.T) {
	mock := &mockNamingClient{}
	d := newMockNacosDiscovery(mock)

	_, err := d.Register(t.Context(), "test-service", "")
	if !errors.Is(err, ErrEmptyAddress) {
		t.Errorf("期望 ErrEmptyAddress，得到 %v", err)
	}
}

func TestNacosDiscovery_RegisterFailed(t *testing.T) {
	mock := &mockNamingClient{
		registerFn: func(param vo.RegisterInstanceParam) (bool, error) {
			return false, errors.New("register error")
		},
	}
	d := newMockNacosDiscovery(mock)

	_, err := d.Register(t.Context(), "test-service", "127.0.0.1:9090")
	if !errors.Is(err, ErrRegister) {
		t.Errorf("期望 ErrRegister，得到 %v", err)
	}
}

func TestNacosDiscovery_Unregister(t *testing.T) {
	mock := &mockNamingClient{}
	d := newMockNacosDiscovery(mock)

	ctx := t.Context()

	serviceID, err := d.Register(ctx, "test-service", "127.0.0.1:9090")
	if err != nil {
		t.Fatalf("注册服务失败: %v", err)
	}

	if err := d.Unregister(ctx, serviceID); err != nil {
		t.Fatalf("注销服务失败: %v", err)
	}
}

func TestNacosDiscovery_UnregisterEmptyID(t *testing.T) {
	mock := &mockNamingClient{}
	d := newMockNacosDiscovery(mock)

	err := d.Unregister(t.Context(), "")
	if !errors.Is(err, ErrEmptyServiceID) {
		t.Errorf("期望 ErrEmptyServiceID，得到 %v", err)
	}
}

func TestNacosDiscovery_UnregisterNotFound(t *testing.T) {
	mock := &mockNamingClient{}
	d := newMockNacosDiscovery(mock)

	// 注销不存在的 serviceID 应成功（幂等）
	err := d.Unregister(t.Context(), "nonexistent-id")
	if err != nil {
		t.Errorf("期望 nil，得到 %v", err)
	}
}

func TestNacosDiscovery_Discover(t *testing.T) {
	mock := &mockNamingClient{
		selectFn: func(param vo.SelectInstancesParam) ([]model.Instance, error) {
			return []model.Instance{
				{Ip: "10.0.0.1", Port: 8080},
				{Ip: "10.0.0.2", Port: 8080},
			}, nil
		},
	}
	d := newMockNacosDiscovery(mock)

	addrs, err := d.Discover(t.Context(), "test-service")
	if err != nil {
		t.Fatalf("发现服务失败: %v", err)
	}
	if len(addrs) != 2 {
		t.Fatalf("期望发现 2 个实例，得到 %d", len(addrs))
	}
	if addrs[0] != "10.0.0.1:8080" {
		t.Errorf("期望 10.0.0.1:8080，得到 %s", addrs[0])
	}
}

func TestNacosDiscovery_DiscoverEmptyName(t *testing.T) {
	mock := &mockNamingClient{}
	d := newMockNacosDiscovery(mock)

	_, err := d.Discover(t.Context(), "")
	if !errors.Is(err, ErrEmptyName) {
		t.Errorf("期望 ErrEmptyName，得到 %v", err)
	}
}

func TestNacosDiscovery_DiscoverError(t *testing.T) {
	mock := &mockNamingClient{
		selectFn: func(param vo.SelectInstancesParam) ([]model.Instance, error) {
			return nil, errors.New("nacos error")
		},
	}
	d := newMockNacosDiscovery(mock)

	_, err := d.Discover(t.Context(), "test-service")
	if !errors.Is(err, ErrDiscover) {
		t.Errorf("期望 ErrDiscover，得到 %v", err)
	}
}

func TestNacosDiscovery_Close(t *testing.T) {
	mock := &mockNamingClient{}
	d := newMockNacosDiscovery(mock)

	if err := d.Close(); err != nil {
		t.Fatalf("Close 失败: %v", err)
	}
}

func TestNacosDiscovery_RegisterAndDiscover(t *testing.T) {
	registeredInstances := make(map[string]vo.RegisterInstanceParam)

	mock := &mockNamingClient{
		registerFn: func(param vo.RegisterInstanceParam) (bool, error) {
			key := param.Ip + ":" + fmt.Sprintf("%d", param.Port)
			registeredInstances[key] = param
			return true, nil
		},
		selectFn: func(param vo.SelectInstancesParam) ([]model.Instance, error) {
			instances := make([]model.Instance, 0, len(registeredInstances))
			for _, reg := range registeredInstances {
				instances = append(instances, model.Instance{
					Ip:   reg.Ip,
					Port: reg.Port,
				})
			}
			return instances, nil
		},
	}
	d := newMockNacosDiscovery(mock)
	ctx := t.Context()

	// 注册服务
	_, err := d.Register(ctx, "my-service", "10.0.0.1:8080")
	if err != nil {
		t.Fatalf("注册服务失败: %v", err)
	}

	// 发现服务
	addrs, err := d.Discover(ctx, "my-service")
	if err != nil {
		t.Fatalf("发现服务失败: %v", err)
	}
	if len(addrs) == 0 {
		t.Fatal("应发现至少一个服务实例")
	}

	found := false
	for _, addr := range addrs {
		if addr == "10.0.0.1:8080" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("未发现注册的服务地址，addrs=%v", addrs)
	}
}
