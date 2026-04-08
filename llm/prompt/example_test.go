package prompt_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/llm"
	"github.com/Tsukikage7/servex/llm/prompt"
)

func ExampleTemplate_Render() {
	tmpl := prompt.MustNew(llm.RoleUser, "Translate '{{.Text}}' to {{.Lang}}.")

	msg, err := tmpl.Render(map[string]string{
		"Text": "hello",
		"Lang": "Chinese",
	})
	fmt.Println(err)
	fmt.Println(msg.Role)
	fmt.Println(msg.Content)
	// Output:
	// <nil>
	// user
	// Translate 'hello' to Chinese.
}
