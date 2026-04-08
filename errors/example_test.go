package errors_test

import (
	"fmt"
	"net/http"

	"google.golang.org/grpc/codes"

	"github.com/Tsukikage7/servex/errors"
)

func ExampleNew() {
	err := errors.New(40001, "INVALID_PARAM", "参数无效").
		WithHTTP(http.StatusBadRequest).
		WithGRPC(codes.InvalidArgument)

	fmt.Println(err.Error())
	fmt.Println(err.Code)
	fmt.Println(err.HTTP)
	fmt.Println(err.GRPC)
	// Output:
	// [40001] INVALID_PARAM: 参数无效
	// 40001
	// 400
	// InvalidArgument
}
