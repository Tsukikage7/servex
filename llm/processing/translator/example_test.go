package translator_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm/processing/translator"
)

func ExampleTranslation() {
	t := translator.Translation{
		Text:           "你好世界",
		SourceLanguage: "en",
		TargetLanguage: "zh",
	}
	fmt.Println(t.TargetLanguage)
	// Output:
	// zh
}
