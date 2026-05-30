package billing_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm/gateway/billing"
)

func ExampleNewMemoryStore() {
	store := billing.NewMemoryStore()
	fmt.Println(store != nil)
	// Output:
	// true
}

func ExamplePriceModel() {
	pm := billing.PriceModel{
		ModelID:         "gpt-4o",
		InputPricePerM:  30.0,
		OutputPricePerM: 60.0,
	}
	fmt.Println(pm.ModelID)
	// Output:
	// gpt-4o
}
