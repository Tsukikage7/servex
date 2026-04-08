package jobqueue_test

import (
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/messaging/jobqueue"
)

func ExampleJob() {
	job := &jobqueue.Job{
		ID:         "job-001",
		Queue:      "default",
		Type:       "send-email",
		Payload:    []byte(`{"to":"user@example.com"}`),
		Priority:   1,
		MaxRetries: 3,
		Status:     jobqueue.StatusPending,
		Delay:      5 * time.Second,
	}
	fmt.Println(job.Type)
	fmt.Println(job.Status)
	fmt.Println(job.MaxRetries)
	// Output:
	// send-email
	// pending
	// 3
}
