// Package redis 提供基于 Redis 的 jobqueue.Store 实现.
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/Tsukikage7/servex/v2/messaging/jobqueue"
)

// Store 基于 Redis 的 jobqueue.Store 实现.
// 使用 sorted set 实现延迟和优先级队列.
type Store struct {
	client *goredis.Client
	opts   options
}

// NewStore 基于 go-redis Client 创建 Redis Store.
func NewStore(client *goredis.Client, opts ...Option) (*Store, error) {
	if client == nil {
		return nil, errors.New("jobqueue/redis: client 不能为空")
	}
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	return &Store{client: client, opts: o}, nil
}

func (s *Store) key(parts ...string) string {
	base := "jobqueue"
	if s.opts.prefix != "" {
		base = s.opts.prefix + ":" + base
	}
	for _, p := range parts {
		base += ":" + p
	}
	return base
}

func (s *Store) Enqueue(ctx context.Context, job *jobqueue.Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}

	// 存储 job 数据
	if err := s.client.Set(ctx, s.key("job", job.ID), data, 0).Err(); err != nil {
		return err
	}

	// 加入 sorted set，score = 优先级反转 + 调度时间
	score := float64(job.ScheduledAt.UnixMilli()) - float64(job.Priority)*1e12
	return s.client.ZAdd(ctx, s.key("queue", job.Queue), goredis.Z{
		Score:  score,
		Member: job.ID,
	}).Err()
}

// dequeueScript 使用 Lua 脚本原子地查询并移除到期的 job，避免 TOCTOU 竞态.
var dequeueScript = goredis.NewScript(`
local result = redis.call("ZRANGEBYSCORE", KEYS[1], "-inf", ARGV[1], "LIMIT", 0, 1)
if #result == 0 then
	return nil
end
local jobID = result[1]
local removed = redis.call("ZREM", KEYS[1], jobID)
if removed == 0 then
	return nil
end
return jobID
`)

func (s *Store) Dequeue(ctx context.Context, queue string) (*jobqueue.Job, error) {
	now := fmt.Sprintf("%f", float64(time.Now().UnixMilli()))
	result, err := dequeueScript.Run(ctx, s.client, []string{s.key("queue", queue)}, now).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, jobqueue.ErrDequeueTimeout
		}
		return nil, err
	}

	jobID, ok := result.(string)
	if !ok || jobID == "" {
		return nil, jobqueue.ErrDequeueTimeout
	}

	data, err := s.client.Get(ctx, s.key("job", jobID)).Bytes()
	if err != nil {
		return nil, err
	}

	var job jobqueue.Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *Store) MarkRunning(ctx context.Context, id string) error {
	return s.updateStatus(ctx, id, jobqueue.StatusRunning, "")
}

func (s *Store) MarkFailed(ctx context.Context, id string, jobErr error) error {
	errMsg := ""
	if jobErr != nil {
		errMsg = jobErr.Error()
	}
	return s.updateStatus(ctx, id, jobqueue.StatusFailed, errMsg)
}

func (s *Store) MarkDead(ctx context.Context, id string) error {
	job, err := s.getJob(ctx, id)
	if err != nil {
		return err
	}
	job.Status = jobqueue.StatusDead
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	pipe.Set(ctx, s.key("job", id), data, 0)
	pipe.SAdd(ctx, s.key("dead", job.Queue), id)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *Store) MarkDone(ctx context.Context, id string) error {
	s.client.Del(ctx, s.key("job", id))
	return nil
}

func (s *Store) Requeue(ctx context.Context, job *jobqueue.Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	if err := s.client.Set(ctx, s.key("job", job.ID), data, 0).Err(); err != nil {
		return err
	}
	score := float64(job.ScheduledAt.UnixMilli()) - float64(job.Priority)*1e12
	return s.client.ZAdd(ctx, s.key("queue", job.Queue), goredis.Z{
		Score:  score,
		Member: job.ID,
	}).Err()
}

func (s *Store) ListDead(ctx context.Context, queue string) ([]*jobqueue.Job, error) {
	ids, err := s.client.SMembers(ctx, s.key("dead", queue)).Result()
	if err != nil {
		return nil, err
	}
	jobs := make([]*jobqueue.Job, 0, len(ids))
	for _, id := range ids {
		job, err := s.getJob(ctx, id)
		if err != nil {
			continue
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (s *Store) Close() error { return nil }

func (s *Store) getJob(ctx context.Context, id string) (*jobqueue.Job, error) {
	data, err := s.client.Get(ctx, s.key("job", id)).Bytes()
	if err != nil {
		return nil, err
	}
	var job jobqueue.Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *Store) updateStatus(ctx context.Context, id string, status jobqueue.Status, lastError string) error {
	job, err := s.getJob(ctx, id)
	if err != nil {
		return err
	}
	job.Status = status
	if lastError != "" {
		job.LastError = lastError
	}
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.key("job", id), data, 0).Err()
}
