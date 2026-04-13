package factory_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/notify/factory"
)

func ExampleConfig() {
	cfg := factory.Config{
		Email: &factory.EmailConfig{
			Host: "smtp.example.com",
			Port: 587,
		},
	}
	fmt.Println(cfg.Email.Host)
	// Output:
	// smtp.example.com
}
