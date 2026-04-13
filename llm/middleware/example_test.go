package middleware_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/middleware"
)

func ExampleChain() {
	// Wrap creates a ChatModel from generate/stream function pair.
	model := middleware.Wrap(
		func(_ context.Context, msgs []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{
				Message:      llm.AssistantMessage("echo:" + msgs[len(msgs)-1].Content),
				FinishReason: "stop",
			}, nil
		},
		func(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (llm.StreamReader, error) {
			return nil, fmt.Errorf("not implemented")
		},
	)

	resp, err := model.Generate(context.Background(), []llm.Message{llm.UserMessage("hi")})
	fmt.Println(err)
	fmt.Println(resp.Message.Content)
	// Output:
	// <nil>
	// echo:hi
}
