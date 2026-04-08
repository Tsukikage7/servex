package moderation_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/llm/safety/moderation"
)

func ExampleNewKeywordModerator() {
	mod := moderation.NewKeywordModerator(map[moderation.Category][]string{
		moderation.CategoryHate: {"badword"},
	})
	result, err := mod.Moderate(context.Background(), "this is fine")
	fmt.Println(err)
	fmt.Println(result.Flagged)
	// Output:
	// <nil>
	// false
}
