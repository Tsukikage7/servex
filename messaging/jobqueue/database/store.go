// Package database 提供基于关系数据库的 jobqueue.Store 实现.
package database

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/v2/messaging/jobqueue"
)

type jobModel struct {
	ID          string `gorm:"primaryKey;size:36"`
	Queue       string `gorm:"index:idx_queue_sched;size:255"`
	Type        string `gorm:"size:255"`
	Payload     []byte
	Headers     string `gorm:"type:text"`
	Priority    int
	MaxRetries  int
	Retried     int
	Status      string `gorm:"index;size:20"`
	LastError   string `gorm:"type:text"`
	CreatedAt   time.Time
	ScheduledAt time.Time `gorm:"index:idx_queue_sched"`
	Deadline    time.Time
}

// Store 基于 GORM 的 jobqueue.Store 实现.
type Store struct {
	db   *gorm.DB
	opts options
}

// NewStore 基于 GORM 数据库连接创建 Store.
func NewStore(db *gorm.DB, opts ...Option) (*Store, error) {
	if db == nil {
		return nil, errors.New("jobqueue/database: db 不能为空")
	}
	o := options{tableName: "jobqueue_jobs"}
	for _, opt := range opts {
		opt(&o)
	}
	s := &Store{db: db, opts: o}
	if err := db.Table(o.tableName).AutoMigrate(&jobModel{}); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) table() *gorm.DB {
	return s.db.Table(s.opts.tableName)
}

func (s *Store) Enqueue(ctx context.Context, job *jobqueue.Job) error {
	m := toModel(job)
	return s.table().WithContext(ctx).Create(&m).Error
}

func (s *Store) Dequeue(ctx context.Context, queue string) (*jobqueue.Job, error) {
	var m jobModel
	result := s.table().WithContext(ctx).
		Where("queue = ? AND status = ? AND scheduled_at <= ?", queue, string(jobqueue.StatusPending), time.Now()).
		Order("priority DESC, scheduled_at ASC").
		Limit(1).
		First(&m)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, jobqueue.ErrDequeueTimeout
		}
		return nil, result.Error
	}

	// 乐观锁：只有 pending 状态才能切换到 running
	result = s.table().WithContext(ctx).
		Where("id = ? AND status = ?", m.ID, string(jobqueue.StatusPending)).
		Update("status", string(jobqueue.StatusRunning))
	if result.RowsAffected == 0 {
		return nil, jobqueue.ErrDequeueTimeout
	}

	return fromModel(&m), nil
}

func (s *Store) MarkRunning(ctx context.Context, id string) error {
	return s.table().WithContext(ctx).Where("id = ?", id).Update("status", string(jobqueue.StatusRunning)).Error
}

func (s *Store) MarkFailed(ctx context.Context, id string, err error) error {
	updates := map[string]any{"status": string(jobqueue.StatusFailed)}
	if err != nil {
		updates["last_error"] = err.Error()
	}
	return s.table().WithContext(ctx).Where("id = ?", id).Updates(updates).Error
}

func (s *Store) MarkDead(ctx context.Context, id string) error {
	return s.table().WithContext(ctx).Where("id = ?", id).Update("status", string(jobqueue.StatusDead)).Error
}

func (s *Store) MarkDone(ctx context.Context, id string) error {
	return s.table().WithContext(ctx).Where("id = ?", id).Delete(&jobModel{}).Error
}

func (s *Store) Requeue(ctx context.Context, job *jobqueue.Job) error {
	m := toModel(job)
	return s.table().WithContext(ctx).Where("id = ?", m.ID).Updates(map[string]any{
		"status":       string(jobqueue.StatusPending),
		"retried":      m.Retried,
		"last_error":   m.LastError,
		"scheduled_at": m.ScheduledAt,
		"headers":      m.Headers,
	}).Error
}

func (s *Store) ListDead(ctx context.Context, queue string) ([]*jobqueue.Job, error) {
	var models []jobModel
	if err := s.table().WithContext(ctx).Where("queue = ? AND status = ?", queue, string(jobqueue.StatusDead)).Find(&models).Error; err != nil {
		return nil, err
	}
	jobs := make([]*jobqueue.Job, len(models))
	for i := range models {
		jobs[i] = fromModel(&models[i])
	}
	return jobs, nil
}

func (s *Store) Close() error { return nil }

func toModel(j *jobqueue.Job) *jobModel {
	var headers string
	if len(j.Headers) > 0 {
		data, _ := json.Marshal(j.Headers)
		headers = string(data)
	}
	return &jobModel{
		ID: j.ID, Queue: j.Queue, Type: j.Type, Payload: j.Payload,
		Headers:  headers,
		Priority: j.Priority, MaxRetries: j.MaxRetries, Retried: j.Retried,
		Status: string(j.Status), LastError: j.LastError,
		CreatedAt: j.CreatedAt, ScheduledAt: j.ScheduledAt, Deadline: j.Deadline,
	}
}

func fromModel(m *jobModel) *jobqueue.Job {
	var headers map[string]string
	if m.Headers != "" {
		_ = json.Unmarshal([]byte(m.Headers), &headers)
	}
	return &jobqueue.Job{
		ID: m.ID, Queue: m.Queue, Type: m.Type, Payload: m.Payload,
		Headers:  headers,
		Priority: m.Priority, MaxRetries: m.MaxRetries, Retried: m.Retried,
		Status: jobqueue.Status(m.Status), LastError: m.LastError,
		CreatedAt: m.CreatedAt, ScheduledAt: m.ScheduledAt, Deadline: m.Deadline,
	}
}
