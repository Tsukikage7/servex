package response_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/transport/response"
)

func ExampleOK() {
	resp := response.OK("hello")
	fmt.Println("code:", resp.Code)
	fmt.Println("message:", resp.Message)
	fmt.Println("data:", resp.Data)
	fmt.Println("success:", resp.IsSuccess())
	// Output:
	// code: 0
	// message: 成功
	// data: hello
	// success: true
}

func ExampleFail() {
	resp := response.Fail[any](response.CodeNotFound)
	fmt.Println("code:", resp.Code)
	fmt.Println("message:", resp.Message)
	fmt.Println("success:", resp.IsSuccess())
	// Output:
	// code: 40001
	// message: 资源不存在
	// success: false
}

func ExampleCode_WithMessage() {
	code := response.CodeInvalidParam.WithMessage("email format invalid")
	fmt.Println("code:", code.Num)
	fmt.Println("message:", code.Message)
	// Output:
	// code: 30001
	// message: email format invalid
}

func ExampleNewCode() {
	custom := response.NewCode(90001, "custom error", 500, 13)
	fmt.Println("num:", custom.Num)
	fmt.Println("message:", custom.Message)
	fmt.Println("http:", custom.HTTPStatus)
	// Output:
	// num: 90001
	// message: custom error
	// http: 500
}
