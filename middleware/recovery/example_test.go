package recovery_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/middleware/recovery"
)

func ExamplePanicError_Error() {
	pe := &recovery.PanicError{
		Value: "something went wrong",
		Stack: []byte("goroutine 1 [running]:"),
	}

	fmt.Println(pe.Error())
	// Output:
	// panic: something went wrong
}

func ExamplePanicError_Unwrap() {
	// When the panic value is an error, Unwrap returns it.
	inner := fmt.Errorf("inner error")
	pe := &recovery.PanicError{Value: inner}

	fmt.Println(pe.Unwrap())
	// Output:
	// inner error
}
