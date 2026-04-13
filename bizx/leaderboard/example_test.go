package leaderboard_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/bizx/leaderboard"
)

func ExampleNewMemoryLeaderboard() {
	lb := leaderboard.NewMemoryLeaderboard("daily_score")
	ctx := context.Background()

	// 提交分数.
	_ = lb.AddScore(ctx, "alice", 100)
	_ = lb.AddScore(ctx, "bob", 250)
	_ = lb.AddScore(ctx, "carol", 180)

	// 获取排行榜 Top 3.
	top, _ := lb.TopN(ctx, 3)
	for _, e := range top {
		fmt.Printf("#%d %s: %.0f\n", e.Rank, e.Member, e.Score)
	}
	// Output:
	// #1 bob: 250
	// #2 carol: 180
	// #3 alice: 100
}

func ExampleLeaderboard_GetRank() {
	lb := leaderboard.NewMemoryLeaderboard("daily_score")
	ctx := context.Background()

	_ = lb.AddScore(ctx, "alice", 100)
	_ = lb.AddScore(ctx, "bob", 250)
	_ = lb.AddScore(ctx, "carol", 180)

	entry, _ := lb.GetRank(ctx, "carol")
	fmt.Printf("carol: rank=%d score=%.0f\n", entry.Rank, entry.Score)
	// Output:
	// carol: rank=2 score=180
}
