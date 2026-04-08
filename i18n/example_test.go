package i18n_test

import (
	"fmt"

	"golang.org/x/text/language"

	"github.com/Tsukikage7/servex/i18n"
)

func ExampleBundle_NewLocalizer() {
	bundle := i18n.NewBundle(language.English)

	bundle.LoadMessages(language.English, map[string]string{
		"hello": "Hello, {{.Name}}!",
	})
	bundle.LoadMessages(language.SimplifiedChinese, map[string]string{
		"hello": "你好, {{.Name}}!",
	})

	loc := bundle.NewLocalizer("zh-CN")
	msg := loc.Translate("hello", map[string]any{"Name": "World"})
	fmt.Println(msg)
	// Output:
	// 你好, World!
}
