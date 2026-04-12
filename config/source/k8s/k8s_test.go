package k8s

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/Tsukikage7/servex/config"
)

func TestSource_LoadConfigMap(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myapp-config",
			Namespace: "default",
		},
		Data: map[string]string{
			"config.yaml": `server:
  port: 8080`,
		},
	})

	src := NewWithClient(client, "myapp-config",
		WithFormat("yaml"),
		WithKey("config.yaml"),
	)

	kvs, err := src.Load()
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if len(kvs) != 1 {
		t.Fatalf("期望 1 个键值对，得到 %d", len(kvs))
	}
	if kvs[0].Key != "config.yaml" {
		t.Errorf("期望 key=config.yaml，得到 %s", kvs[0].Key)
	}
	if kvs[0].Format != "yaml" {
		t.Errorf("期望 format=yaml，得到 %s", kvs[0].Format)
	}
	if string(kvs[0].Value) != "server:\n  port: 8080" {
		t.Errorf("值不匹配: %s", kvs[0].Value)
	}
}

func TestSource_LoadConfigMapAllKeys(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myapp-config",
			Namespace: "default",
		},
		Data: map[string]string{
			"db.json":    `{"host":"localhost"}`,
			"cache.json": `{"ttl":300}`,
		},
	})

	src := NewWithClient(client, "myapp-config")

	kvs, err := src.Load()
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if len(kvs) != 2 {
		t.Fatalf("期望 2 个键值对，得到 %d", len(kvs))
	}
}

func TestSource_LoadSecret(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myapp-secret",
			Namespace: "production",
		},
		Data: map[string][]byte{
			"credentials.json": []byte(`{"username":"admin","password":"s3cret"}`),
		},
	})

	src := NewWithClient(client, "myapp-secret",
		WithNamespace("production"),
		WithResourceType(ResourceSecret),
		WithKey("credentials.json"),
	)

	kvs, err := src.Load()
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if len(kvs) != 1 {
		t.Fatalf("期望 1 个键值对，得到 %d", len(kvs))
	}
	if string(kvs[0].Value) != `{"username":"admin","password":"s3cret"}` {
		t.Errorf("值不匹配: %s", kvs[0].Value)
	}
}

func TestSource_LoadMissingResource(t *testing.T) {
	client := fake.NewSimpleClientset()

	src := NewWithClient(client, "nonexistent")

	_, err := src.Load()
	if err == nil {
		t.Error("不存在的资源应返回错误")
	}
}

func TestSource_LoadMissingKey(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myapp-config",
			Namespace: "default",
		},
		Data: map[string]string{
			"existing.json": `{}`,
		},
	})

	src := NewWithClient(client, "myapp-config", WithKey("missing.json"))

	_, err := src.Load()
	if err == nil {
		t.Error("不存在的 key 应返回错误")
	}
}

func TestSource_LoadEmptyData(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "empty-config",
			Namespace: "default",
		},
		Data: map[string]string{},
	})

	src := NewWithClient(client, "empty-config")

	_, err := src.Load()
	if err != config.ErrSourceLoad {
		t.Errorf("空数据应返回 ErrSourceLoad，得到 %v", err)
	}
}

func TestSource_Options(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "staging",
		},
		Data: map[string]string{
			"app.toml": `[server]\nport = 9090`,
		},
	})

	src := NewWithClient(client, "test-config",
		WithNamespace("staging"),
		WithFormat("toml"),
		WithKey("app.toml"),
		WithResourceType(ResourceConfigMap),
	)

	if src.namespace != "staging" {
		t.Errorf("期望 namespace=staging，得到 %s", src.namespace)
	}
	if src.format != "toml" {
		t.Errorf("期望 format=toml，得到 %s", src.format)
	}
	if src.key != "app.toml" {
		t.Errorf("期望 key=app.toml，得到 %s", src.key)
	}
	if src.resourceType != ResourceConfigMap {
		t.Errorf("期望 resourceType=ResourceConfigMap，得到 %d", src.resourceType)
	}

	kvs, err := src.Load()
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if kvs[0].Format != "toml" {
		t.Errorf("期望 format=toml，得到 %s", kvs[0].Format)
	}
}

func TestSource_Watch(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-config",
			Namespace: "default",
		},
		Data: map[string]string{
			"config.json": `{"v":1}`,
		},
	})

	src := NewWithClient(client, "watch-config", WithKey("config.json"))

	watcher, err := src.Watch()
	if err != nil {
		t.Fatalf("Watch 失败: %v", err)
	}

	// 在后台更新 ConfigMap 以触发 Watch 事件
	go func() {
		time.Sleep(100 * time.Millisecond)
		_, _ = client.CoreV1().ConfigMaps("default").Update(
			t.Context(),
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "watch-config",
					Namespace: "default",
				},
				Data: map[string]string{
					"config.json": `{"v":2}`,
				},
			},
			metav1.UpdateOptions{},
		)
	}()

	// 使用超时保护，避免测试挂起
	done := make(chan struct{})
	go func() {
		defer close(done)
		kvs, err := watcher.Next()
		if err != nil {
			t.Errorf("Next 失败: %v", err)
			return
		}
		if len(kvs) == 0 {
			t.Error("Next 应返回配置值")
			return
		}
		if string(kvs[0].Value) != `{"v":2}` {
			t.Errorf("期望 {\"v\":2}，得到 %s", kvs[0].Value)
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Error("Watch 超时")
	}

	if err := watcher.Stop(); err != nil {
		t.Errorf("Stop 失败: %v", err)
	}
}

func TestSource_WatchStop(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stop-config",
			Namespace: "default",
		},
		Data: map[string]string{
			"config.json": `{}`,
		},
	})

	src := NewWithClient(client, "stop-config")

	watcher, err := src.Watch()
	if err != nil {
		t.Fatalf("Watch 失败: %v", err)
	}

	// 立即停止
	if err := watcher.Stop(); err != nil {
		t.Errorf("Stop 失败: %v", err)
	}

	// Next 应返回 ErrSourceClosed
	_, err = watcher.Next()
	if err != config.ErrSourceClosed {
		t.Errorf("Stop 后 Next 应返回 ErrSourceClosed，得到 %v", err)
	}
}

func TestSource_SecretWatch(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watch-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"token": []byte("old-token"),
		},
	})

	src := NewWithClient(client, "watch-secret",
		WithResourceType(ResourceSecret),
		WithKey("token"),
	)

	watcher, err := src.Watch()
	if err != nil {
		t.Fatalf("Watch 失败: %v", err)
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		_, _ = client.CoreV1().Secrets("default").Update(
			t.Context(),
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "watch-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"token": []byte("new-token"),
				},
			},
			metav1.UpdateOptions{},
		)
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		kvs, err := watcher.Next()
		if err != nil {
			t.Errorf("Next 失败: %v", err)
			return
		}
		if len(kvs) == 0 {
			t.Error("Next 应返回配置值")
			return
		}
		if string(kvs[0].Value) != "new-token" {
			t.Errorf("期望 new-token，得到 %s", kvs[0].Value)
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Error("Watch 超时")
	}

	if err := watcher.Stop(); err != nil {
		t.Errorf("Stop 失败: %v", err)
	}
}

func TestNewWithConfig_NilConfig(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Error("nil config 应返回错误")
	}
}

func TestNewWithConfig_EmptyName(t *testing.T) {
	_, err := New(&Config{})
	if err == nil {
		t.Error("空 name 应返回错误")
	}
}
