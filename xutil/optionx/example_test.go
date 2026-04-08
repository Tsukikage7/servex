package optionx_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/xutil/optionx"
)

type serverConfig struct {
	Host string
	Port int
}

func ExampleApply() {
	cfg := &serverConfig{Host: "localhost", Port: 8080}
	optionx.Apply(cfg,
		func(c *serverConfig) { c.Host = "0.0.0.0" },
		func(c *serverConfig) { c.Port = 9090 },
	)
	fmt.Println(cfg.Host)
	fmt.Println(cfg.Port)
	// Output:
	// 0.0.0.0
	// 9090
}
