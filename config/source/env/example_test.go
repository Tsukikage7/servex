package env_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/config/source/env"
)

func ExampleNew() {
	src := env.New(env.WithPrefix("MYAPP_"))
	kvs, err := src.Load()
	// Load reads os.Environ; prefix-filtered keys are returned as JSON.
	fmt.Println(err)
	fmt.Println(kvs[0].Key)
	fmt.Println(kvs[0].Format)
	// Output:
	// <nil>
	// env
	// json
}
