package etcd

import (
	"context"
	"os"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// 集成测试：需要本地运行的 etcd 实例.
// 如果 etcd 不可用，测试将自动跳过.

func newTestClient(t *testing.T) *clientv3.Client {
	t.Helper()

	endpoint := "127.0.0.1:2379"
	if ep := os.Getenv("ETCD_ENDPOINTS"); ep != "" {
		endpoint = ep
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{endpoint},
		DialTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Skipf("etcd 不可用（%v），跳过集成测试", err)
	}

	// etcd client 构造不会立即连接，需探测连通性
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if _, err = client.Status(ctx, endpoint); err != nil {
		client.Close()
		t.Skipf("etcd 不可用（%v），跳过集成测试", err)
	}

	return client
}

func TestSource_LoadAndWatch(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	const testKey = "/test/config/source"
	const testValue = `{"host":"localhost","port":8080}`

	ctx := t.Context()

	// 写入初始值
	if _, err := client.Put(ctx, testKey, testValue); err != nil {
		t.Skipf("etcd 不可用（%v），跳过集成测试", err)
	}
	defer client.Delete(ctx, testKey) //nolint

	src := New(client, testKey, WithFormat("json"))

	// 测试 Load
	kvs, err := src.Load()
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if len(kvs) == 0 {
		t.Fatal("Load 应返回配置值")
	}
	if string(kvs[0].Value) != testValue {
		t.Errorf("期望 %s，得到 %s", testValue, kvs[0].Value)
	}
	if kvs[0].Format != "json" {
		t.Errorf("期望格式 json，得到 %s", kvs[0].Format)
	}
}

func TestSource_LoadNotFound(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	src := New(client, "/test/nonexistent/key/12345")
	_, err := src.Load()
	if err == nil {
		t.Error("不存在的 key 应返回错误")
	}
}
