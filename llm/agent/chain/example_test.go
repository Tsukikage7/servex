package chain_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm/agent/chain"
)

func ExampleNew() {
	c := chain.New()
	fmt.Println(c != nil)
	// Output:
	// true
}
