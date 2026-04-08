package copier_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/xutil/copier"
)

type srcUser struct {
	Name  string
	Email string
	Age   int
}

type dstUser struct {
	Name  string
	Email string
	Age   int
}

func ExampleCopy() {
	src := &srcUser{Name: "Alice", Email: "alice@example.com", Age: 30}
	dst, err := copier.Copy[dstUser](src)
	fmt.Println(err)
	fmt.Println(dst.Name)
	fmt.Println(dst.Email)
	fmt.Println(dst.Age)
	// Output:
	// <nil>
	// Alice
	// alice@example.com
	// 30
}
