package llm_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm"
)

func ExampleSystemMessage() {
	msg := llm.SystemMessage("You are a helpful assistant.")
	fmt.Println(msg.Role)
	fmt.Println(msg.Content)
	// Output:
	// system
	// You are a helpful assistant.
}

func ExampleUserMessage() {
	msg := llm.UserMessage("Hello!")
	fmt.Println(msg.Role)
	fmt.Println(msg.Content)
	// Output:
	// user
	// Hello!
}
