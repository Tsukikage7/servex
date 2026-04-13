package conversation_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm/agent/conversation"
)

func ExampleNewBufferMemory() {
	mem := conversation.NewBufferMemory()
	fmt.Println(len(mem.Messages()))
	// Output:
	// 0
}

func ExampleNewWindowMemory() {
	mem := conversation.NewWindowMemory(5)
	fmt.Println(len(mem.Messages()))
	// Output:
	// 0
}
