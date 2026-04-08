package classifier_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/llm/processing/classifier"
)

func ExampleLabel() {
	label := classifier.Label{
		Name:  "positive",
		Score: 0.95,
	}
	fmt.Println(label.Name)
	fmt.Printf("%.2f\n", label.Score)
	// Output:
	// positive
	// 0.95
}
