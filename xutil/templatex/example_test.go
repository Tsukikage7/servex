package templatex_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/xutil/templatex"
)

func ExampleEngine_RenderString() {
	engine := templatex.New()

	result, err := engine.RenderString("Hello, {{.Name}}! You have {{.Count}} items.", map[string]any{
		"Name":  "Alice",
		"Count": 3,
	})
	fmt.Println(err)
	fmt.Println(result)
	// Output:
	// <nil>
	// Hello, Alice! You have 3 items.
}
