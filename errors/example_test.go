package errors_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/errors"
)

func ExampleNew() {
	err := errors.NewWithKind(40001, "INVALID_PARAM", "参数无效", errors.KindInvalidArgument)

	fmt.Println(err.Error())
	fmt.Println(err.Code)
	fmt.Println(err.Kind.HTTPStatus())
	// Output:
	// [40001] INVALID_PARAM: 参数无效
	// 40001
	// 400
}
