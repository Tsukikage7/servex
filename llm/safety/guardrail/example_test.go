package guardrail_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/safety/guardrail"
)

func ExampleMaxLength() {
	guard := guardrail.MaxLength(100)
	err := guard.Check(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "Hello"},
	})
	fmt.Println(err)
	// Output:
	// <nil>
}

func ExampleMaxMessages() {
	guard := guardrail.MaxMessages(10)
	err := guard.Check(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "Hi"},
	})
	fmt.Println(err)
	// Output:
	// <nil>
}
