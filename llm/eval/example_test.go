package eval_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm/eval"
)

func ExampleScore() {
	score := eval.Score{
		Name:   "relevance",
		Value:  0.95,
		Reason: "The answer is highly relevant to the question.",
	}
	fmt.Println(score.Name)
	fmt.Println(score.Value)
	// Output:
	// relevance
	// 0.95
}

func ExampleEvalInput() {
	input := eval.EvalInput{
		Question:  "What is Go?",
		Answer:    "Go is a programming language.",
		Reference: "Go is a statically typed, compiled programming language.",
		Context:   []string{"Go was designed at Google."},
	}
	fmt.Println(input.Question)
	fmt.Println(len(input.Context))
	// Output:
	// What is Go?
	// 1
}
