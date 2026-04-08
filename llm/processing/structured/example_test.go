package structured_test

import (
	"encoding/json"
	"fmt"

	"github.com/Tsukikage7/servex/llm/processing/structured"
)

func ExampleSchemaFrom() {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	schema := structured.SchemaFrom[Person]()
	var m map[string]any
	_ = json.Unmarshal(schema, &m)
	fmt.Println(m["type"])
	// Output:
	// object
}
