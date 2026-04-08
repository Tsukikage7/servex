package cqrs_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/domain/cqrs"
)

// createUserHandler 示例命令处理器.
type createUserHandler struct{}

func (h *createUserHandler) Handle(_ context.Context, cmd string) (string, string, error) {
	return cmd, "user-created:" + cmd, nil
}

func ExampleChainCommand() {
	handler := &createUserHandler{}
	_, result, err := cqrs.ApplyCommand(context.Background(), "alice", handler)
	fmt.Println(result)
	fmt.Println(err)
	// Output:
	// user-created:alice
	// <nil>
}

// getUserHandler 示例查询处理器.
type getUserHandler struct{}

func (h *getUserHandler) Handle(_ context.Context, query string) (string, error) {
	return "found:" + query, nil
}

func ExampleApplyQueryHandler() {
	handler := &getUserHandler{}
	result, err := cqrs.ApplyQueryHandler(context.Background(), "bob", handler)
	fmt.Println(result)
	fmt.Println(err)
	// Output:
	// found:bob
	// <nil>
}
