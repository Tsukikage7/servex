package email_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/notify/email"
)

func ExampleWithSMTP() {
	opt := email.WithSMTP("smtp.example.com", 587)
	fmt.Println(opt != nil)
	// Output:
	// true
}

func ExampleWithFrom() {
	opt := email.WithFrom("noreply@example.com", "MyApp")
	fmt.Println(opt != nil)
	// Output:
	// true
}
