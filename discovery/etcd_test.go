package discovery

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/testx"
)

// 集成测试：需要本地运行的 etcd 实例.
// 可通过环境变量 ETCD_ENDPOINTS 指定地址（逗号分隔）.
// 如果 etcd 不可用，测试将自动跳过.

func skipIfNoEtcd(t *testing.T) ([]string, logger.Logger) {
	t.Helper()

	endpoints := []string{"127.0.0.1:2379"}
	if ep := os.Getenv("ETCD_ENDPOINTS"); ep != "" {
		endpoints = []string{ep}
	}

	log := testx.NopLogger()

	cfg := &Config{
		Type:            TypeEtcd,
		EtcdEndpoints:   endpoints,
		EtcdDialTimeout: 2 * time.Second,
	}
	cfg.SetDefaults()

	// 尝试创建客户端并探测连通性（etcd client 为懒连接，需实际调用 Status）
	d, err := newEtcdDiscovery(cfg, log)
	if err != nil {
		t.Skipf("etcd 不可用（%v），跳过集成测试", err)
	}
	cli := d.(*etcdDiscovery).client
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if _, err = cli.Status(ctx, endpoints[0]); err != nil {
		_ = cli.Close()
		t.Skipf("etcd 不可用（%v），跳过集成测试", err)
	}
	_ = cli.Close()

	return endpoints, log
}

func TestEtcdDiscovery_RegisterAndDiscover(t *testing.T) {
	endpoints, log := skipIfNoEtcd(t)

	cfg := &Config{
		Type:            TypeEtcd,
		EtcdEndpoints:   endpoints,
		EtcdDialTimeout: 2 * time.Second,
	}
	cfg.SetDefaults()

	d, err := newEtcdDiscovery(cfg, log)
	if err != nil {
		t.Fatalf("创建etcd服务发现失败: %v", err)
	}
	defer d.Close()

	ctx := t.Context()
	const serviceName = "test-etcd-service"

	// 注册服务
	serviceID, err := d.Register(ctx, serviceName, "127.0.0.1:9090")
	if err != nil {
		t.Fatalf("注册服务失败: %v", err)
	}
	if serviceID == "" {
		t.Fatal("serviceID 不应为空")
	}

	// 发现服务
	addrs, err := d.Discover(ctx, serviceName)
	if err != nil {
		t.Fatalf("发现服务失败: %v", err)
	}
	if len(addrs) == 0 {
		t.Fatal("应发现至少一个服务实例")
	}

	found := false
	for _, addr := range addrs {
		if addr == "127.0.0.1:9090" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("未发现注册的服务地址，addrs=%v", addrs)
	}

	// 注销服务
	if err := d.Unregister(ctx, serviceID); err != nil {
		t.Fatalf("注销服务失败: %v", err)
	}

	// 验证已注销
	addrs, err = d.Discover(ctx, serviceName)
	if err != nil {
		t.Fatalf("注销后发现服务失败: %v", err)
	}
	for _, addr := range addrs {
		if addr == "127.0.0.1:9090" {
			t.Error("服务应已注销，但仍能发现")
		}
	}
}

func TestEtcdDiscovery_EmptyName(t *testing.T) {
	endpoints, log := skipIfNoEtcd(t)

	cfg := &Config{
		Type:            TypeEtcd,
		EtcdEndpoints:   endpoints,
		EtcdDialTimeout: 2 * time.Second,
	}
	cfg.SetDefaults()

	d, err := newEtcdDiscovery(cfg, log)
	if err != nil {
		t.Fatalf("创建etcd服务发现失败: %v", err)
	}
	defer d.Close()

	ctx := t.Context()

	if _, err := d.Register(ctx, "", "127.0.0.1:9090"); err != ErrEmptyName {
		t.Errorf("期望 ErrEmptyName，得到 %v", err)
	}
	if _, err := d.Discover(ctx, ""); err != ErrEmptyName {
		t.Errorf("期望 ErrEmptyName，得到 %v", err)
	}
}
