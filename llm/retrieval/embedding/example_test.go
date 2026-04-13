package embedding_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm/retrieval/embedding"
)

func ExampleCosineSimilarity() {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	sim := embedding.CosineSimilarity(a, b)
	fmt.Printf("%.1f\n", sim)
	// Output:
	// 1.0
}
