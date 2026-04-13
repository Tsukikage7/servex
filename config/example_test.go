package config_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/config"
)

// 演示用配置源.
type exampleSource struct {
	data   []byte
	format string
}

func (s *exampleSource) Load() ([]*config.KeyValue, error) {
	return []*config.KeyValue{{Key: "example", Value: s.data, Format: s.format}}, nil
}

func (s *exampleSource) Watch() (config.Watcher, error) {
	return nil, config.ErrSourceWatch
}

type exampleConfig struct {
	Name string `json:"name"`
	Port int    `json:"port"`
}

func ExampleNewManager() {
	src := &exampleSource{
		data:   []byte(`{"name":"my-app","port":8080}`),
		format: "json",
	}

	mgr, _ := config.NewManager[exampleConfig](
		config.WithSource[exampleConfig](src),
	)

	_ = mgr.Load()

	cfg := mgr.Get()
	fmt.Println(cfg.Name)
	fmt.Println(cfg.Port)
	// Output:
	// my-app
	// 8080
}

func ExampleGetConfigType() {
	fmt.Println(config.GetConfigType("app.yaml"))
	fmt.Println(config.GetConfigType("config.json"))
	fmt.Println(config.GetConfigType("settings.toml"))
	fmt.Println(config.GetConfigType("app.ini"))
	fmt.Println(config.GetConfigType("unknown.txt"))
	// Output:
	// yaml
	// json
	// toml
	// ini
	//
}

func ExampleNewManager_withObserver() {
	src := &exampleSource{
		data:   []byte(`{"name":"v1","port":3000}`),
		format: "json",
	}

	mgr, _ := config.NewManager[exampleConfig](
		config.WithSource[exampleConfig](src),
		config.WithObserver[exampleConfig](func(old, new *exampleConfig) {
			fmt.Printf("config changed: %s -> %s\n", old.Name, new.Name)
		}),
	)

	_ = mgr.Load()
	fmt.Println(mgr.Get().Name)
	// Output: v1
}

func ExampleConfigFieldError_Error() {
	err := &config.ConfigFieldError{
		Field:    "database.host",
		Source:   "file:config.yaml",
		Message:  "类型不匹配",
		Expected: "string",
		Actual:   "123",
	}
	fmt.Println(err.Error())
	// Output: config: 字段 "database.host" (来源: file:config.yaml): 类型不匹配 (期望: string, 实际: 123)
}

func ExampleNewFieldError() {
	err := config.NewFieldError("server.port", "env:SERVER_PORT", "字段缺失")
	fmt.Println(err.Error())
	// Output: config: 字段 "server.port" (来源: env:SERVER_PORT): 字段缺失
}

func ExampleNewFieldTypeError() {
	err := config.NewFieldTypeError("server.port", "file:app.yaml", "int", "\"abc\"")
	fmt.Println(err.Error())
	// Output: config: 字段 "server.port" (来源: file:app.yaml): 类型不匹配 (期望: int, 实际: "abc")
}
