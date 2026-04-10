package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/testx"
)

func TestNewScheduler(t *testing.T) {
	s, err := NewScheduler()
	if err != nil {
		t.Fatalf("NewScheduler error: %v", err)
	}
	if s == nil {
		t.Fatal("scheduler is nil")
	}
	if s.Running() {
		t.Fatal("scheduler should not be running before Start")
	}
}

func TestJobValidation(t *testing.T) {
	tests := []struct {
		name    string
		job     *Job
		wantErr error
	}{
		{"empty name", &Job{Schedule: "* * * * * *", Handler: func(ctx context.Context) error { return nil }}, ErrJobNameEmpty},
		{"empty schedule", &Job{Name: "test", Handler: func(ctx context.Context) error { return nil }}, ErrScheduleEmpty},
		{"nil handler", &Job{Name: "test", Schedule: "* * * * * *"}, ErrHandlerNil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.job.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestJobBuilder(t *testing.T) {
	job, err := NewJob("test-job").
		Schedule("* * * * * *").
		Handler(func(ctx context.Context) error { return nil }).
		Timeout(time.Second).
		Singleton().
		Retry(3, 100*time.Millisecond).
		Build()

	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if job.Name != "test-job" {
		t.Errorf("Name = %q", job.Name)
	}
	if !job.Singleton {
		t.Error("expected Singleton=true")
	}
	if job.RetryCount != 3 {
		t.Errorf("RetryCount = %d", job.RetryCount)
	}
}

func TestJobBuilderMustBuildPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from MustBuild with invalid job")
		}
	}()
	NewJob("").MustBuild()
}

func TestJobState(t *testing.T) {
	job := &Job{Name: "state-test"}
	if job.State() != JobStateIdle {
		t.Fatalf("initial state = %v, want Idle", job.State())
	}
	if job.IsRunning() {
		t.Fatal("should not be running initially")
	}

	if !job.tryStart() {
		t.Fatal("tryStart should succeed")
	}
	if !job.IsRunning() {
		t.Fatal("should be running after tryStart")
	}
	if job.tryStart() {
		t.Fatal("second tryStart should fail")
	}

	job.finish()
	if job.IsRunning() {
		t.Fatal("should not be running after finish")
	}
}

func TestJobStateString(t *testing.T) {
	tests := []struct {
		state JobState
		want  string
	}{
		{JobStateIdle, "idle"},
		{JobStateRunning, "running"},
		{JobStatePaused, "paused"},
		{JobState(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("JobState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestJobStats(t *testing.T) {
	job := &Job{Name: "stats-test", Schedule: "* * * * * *", Handler: func(ctx context.Context) error { return nil }}
	job.initStats()

	stats := job.Stats()
	if stats.RunCount != 0 {
		t.Errorf("RunCount = %d, want 0", stats.RunCount)
	}

	job.stats.recordStart()
	job.stats.recordSuccess(50 * time.Millisecond)

	stats = job.Stats()
	if stats.RunCount != 1 {
		t.Errorf("RunCount = %d, want 1", stats.RunCount)
	}
	if stats.SuccessCount != 1 {
		t.Errorf("SuccessCount = %d, want 1", stats.SuccessCount)
	}
}

func TestSchedulerAddAndGet(t *testing.T) {
	s, err := NewScheduler(WithLogger(testx.NopLogger()))
	if err != nil {
		t.Fatalf("NewScheduler error: %v", err)
	}

	job := &Job{
		Name:     "add-test",
		Schedule: "* * * * * *",
		Handler:  func(ctx context.Context) error { return nil },
	}

	if err := s.Add(job); err != nil {
		t.Fatalf("Add error: %v", err)
	}

	got, ok := s.Get("add-test")
	if !ok {
		t.Fatal("Get returned false")
	}
	if got.Name != "add-test" {
		t.Errorf("Name = %q", got.Name)
	}

	// Duplicate add should fail.
	err = s.Add(&Job{Name: "add-test", Schedule: "* * * * * *", Handler: func(ctx context.Context) error { return nil }})
	if !errors.Is(err, ErrJobExists) {
		t.Fatalf("expected ErrJobExists, got %v", err)
	}
}

func TestSchedulerStartStopAndTrigger(t *testing.T) {
	s, err := NewScheduler(WithLogger(testx.NopLogger()))
	if err != nil {
		t.Fatalf("NewScheduler error: %v", err)
	}

	var count atomic.Int32
	job := &Job{
		Name:     "trigger-test",
		Schedule: "0 0 0 1 1 *", // far future, won't fire naturally
		Handler: func(ctx context.Context) error {
			count.Add(1)
			return nil
		},
	}

	if err := s.Add(job); err != nil {
		t.Fatalf("Add error: %v", err)
	}

	if err := s.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer s.Stop()

	if !s.Running() {
		t.Fatal("expected Running() = true")
	}

	if err := s.Trigger("trigger-test"); err != nil {
		t.Fatalf("Trigger error: %v", err)
	}

	// TODO: 考虑使用 testing/synctest 替换 time.Sleep，
	// 当前 cronScheduler 内部依赖 robfig/cron 的后台 goroutine，
	// 无法完全封装在 synctest bubble 中.
	// Wait briefly for the triggered job to complete.
	time.Sleep(50 * time.Millisecond)

	if count.Load() < 1 {
		t.Fatal("expected job to have been triggered at least once")
	}
}

func TestSchedulerRemove(t *testing.T) {
	s, err := NewScheduler(WithLogger(testx.NopLogger()))
	if err != nil {
		t.Fatalf("NewScheduler error: %v", err)
	}

	job := &Job{
		Name:     "remove-test",
		Schedule: "* * * * * *",
		Handler:  func(ctx context.Context) error { return nil },
	}
	_ = s.Add(job)

	if err := s.Remove("remove-test"); err != nil {
		t.Fatalf("Remove error: %v", err)
	}

	_, ok := s.Get("remove-test")
	if ok {
		t.Fatal("expected job to be removed")
	}

	err = s.Remove("nonexistent")
	if !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("expected ErrJobNotFound, got %v", err)
	}
}

func TestSchedulerList(t *testing.T) {
	s, err := NewScheduler()
	if err != nil {
		t.Fatalf("NewScheduler error: %v", err)
	}

	_ = s.Add(&Job{Name: "j1", Schedule: "* * * * * *", Handler: func(ctx context.Context) error { return nil }})
	_ = s.Add(&Job{Name: "j2", Schedule: "* * * * * *", Handler: func(ctx context.Context) error { return nil }})

	list := s.List()
	if len(list) != 2 {
		t.Fatalf("List() len = %d, want 2", len(list))
	}
}

func TestSchedulerShutdown(t *testing.T) {
	s, err := NewScheduler(WithLogger(testx.NopLogger()))
	if err != nil {
		t.Fatalf("NewScheduler error: %v", err)
	}

	_ = s.Start()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown error: %v", err)
	}
}
