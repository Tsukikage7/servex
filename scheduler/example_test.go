package scheduler_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/scheduler"
)

func ExampleNewJob() {
	job, err := scheduler.NewJob("sync-data").
		Schedule("0 */5 * * * *").
		Handler(func(_ context.Context) error {
			return nil
		}).
		Singleton().
		Build()
	fmt.Println(err)
	fmt.Println(job.Name)
	fmt.Println(job.Schedule)
	fmt.Println(job.Singleton)
	// Output:
	// <nil>
	// sync-data
	// 0 */5 * * * *
	// true
}
