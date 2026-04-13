package graphql_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/transport/graphql"
)

func ExampleDefaultConfig() {
	cfg := graphql.DefaultConfig()
	fmt.Println(cfg != nil)
	// Output:
	// true
}
