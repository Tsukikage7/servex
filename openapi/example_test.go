package openapi_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/openapi"
)

func ExampleNewRegistry() {
	reg := openapi.NewRegistry(
		openapi.WithInfo("Pet Store API", "1.0.0", "A sample API"),
		openapi.WithServer("https://api.example.com"),
	)

	reg.Add(
		openapi.GET("/pets").
			Summary("List all pets").
			Tags("pets").
			OperationID("listPets").
			Build(),
	)

	spec := reg.Build()
	fmt.Println(spec.OpenAPI)
	fmt.Println(spec.Info.Title)
	fmt.Println(spec.Info.Version)
	// Output:
	// 3.0.3
	// Pet Store API
	// 1.0.0
}
