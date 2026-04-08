package validation_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/validation"
)

func ExampleValidator_Validate() {
	v := validation.New()

	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	err := v.Validate(&User{Name: "Alice", Email: "alice@example.com"})
	fmt.Println(err)
	// Output:
	// <nil>
}

func ExampleValidator_Validate_error() {
	v := validation.New()

	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	err := v.Validate(&User{})
	fmt.Println(err)
	// Output:
	// validation failed: name (required), email (required)
}
