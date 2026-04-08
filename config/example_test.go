package config_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/config"
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
