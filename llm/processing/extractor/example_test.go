package extractor_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/llm/processing/extractor"
)

func ExampleEntity() {
	entity := extractor.Entity{
		Text:  "Beijing",
		Type:  "Location",
		Start: 0,
		End:   7,
	}
	fmt.Println(entity.Text)
	fmt.Println(entity.Type)
	// Output:
	// Beijing
	// Location
}
