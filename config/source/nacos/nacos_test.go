package nacos

import (
	"errors"
	"testing"

	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"

	"github.com/Tsukikage7/servex/v2/config"
)

// mockConfigClient 模拟 Nacos config client.
type mockConfigClient struct {
	content  string
	getErr   error
	listenFn func(params vo.ConfigParam) error
}

func (m *mockConfigClient) GetConfig(param vo.ConfigParam) (string, error) {
	return m.content, m.getErr
}

func (m *mockConfigClient) PublishConfig(param vo.ConfigParam) (bool, error) {
	return true, nil
}

func (m *mockConfigClient) DeleteConfig(param vo.ConfigParam) (bool, error) {
	return true, nil
}

func (m *mockConfigClient) ListenConfig(param vo.ConfigParam) error {
	if m.listenFn != nil {
		return m.listenFn(param)
	}
	return nil
}

func (m *mockConfigClient) CancelListenConfig(param vo.ConfigParam) error {
	return nil
}

func (m *mockConfigClient) SearchConfig(param vo.SearchConfigParam) (*model.ConfigPage, error) {
	return nil, nil
}

func (m *mockConfigClient) CloseClient() {}

func TestSource_Load(t *testing.T) {
	const testDataID = "app.json"
	const testContent = `{"host":"localhost","port":8080}`

	mock := &mockConfigClient{content: testContent}
	src := New(mock, testDataID, WithFormat("json"), WithGroup("TEST_GROUP"), WithNamespace("test-ns"))

	kvs, err := src.Load()
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if len(kvs) == 0 {
		t.Fatal("Load 应返回配置值")
	}
	if string(kvs[0].Value) != testContent {
		t.Errorf("期望 %s，得到 %s", testContent, kvs[0].Value)
	}
	if kvs[0].Key != testDataID {
		t.Errorf("期望 key %s，得到 %s", testDataID, kvs[0].Key)
	}
	if kvs[0].Format != "json" {
		t.Errorf("期望格式 json，得到 %s", kvs[0].Format)
	}
}

func TestSource_LoadEmpty(t *testing.T) {
	mock := &mockConfigClient{content: ""}
	src := New(mock, "empty.json")

	_, err := src.Load()
	if !errors.Is(err, config.ErrSourceLoad) {
		t.Errorf("期望 ErrSourceLoad，得到 %v", err)
	}
}

func TestSource_LoadError(t *testing.T) {
	mock := &mockConfigClient{getErr: errors.New("nacos error")}
	src := New(mock, "fail.json")

	_, err := src.Load()
	if err == nil {
		t.Error("期望错误，得到 nil")
	}
}

func TestSource_Watch(t *testing.T) {
	var onChange func(namespace, group, dataId, data string)

	mock := &mockConfigClient{
		listenFn: func(param vo.ConfigParam) error {
			onChange = param.OnChange
			return nil
		},
	}
	src := New(mock, "app.json")

	watcher, err := src.Watch()
	if err != nil {
		t.Fatalf("Watch 失败: %v", err)
	}

	// 模拟配置变更
	if onChange != nil {
		onChange("", "DEFAULT_GROUP", "app.json", `{"updated":true}`)
	}

	kvs, err := watcher.Next()
	if err != nil {
		t.Fatalf("Next 失败: %v", err)
	}
	if string(kvs[0].Value) != `{"updated":true}` {
		t.Errorf("期望更新后的配置，得到 %s", kvs[0].Value)
	}

	if err := watcher.Stop(); err != nil {
		t.Fatalf("Stop 失败: %v", err)
	}
}

func TestSource_WatchListenError(t *testing.T) {
	mock := &mockConfigClient{
		listenFn: func(param vo.ConfigParam) error {
			return errors.New("listen error")
		},
	}
	src := New(mock, "app.json")

	_, err := src.Watch()
	if err == nil {
		t.Error("期望错误，得到 nil")
	}
}

func TestSource_DefaultOptions(t *testing.T) {
	mock := &mockConfigClient{content: "data"}
	src := New(mock, "test.json")

	if src.group != "DEFAULT_GROUP" {
		t.Errorf("期望默认 group DEFAULT_GROUP，得到 %s", src.group)
	}
	if src.format != "json" {
		t.Errorf("期望默认 format json，得到 %s", src.format)
	}
}
