// Package leaderboard 提供排行榜实现.
// 区别于 storage/redis 的 ZSet 操作，本包提供语义化的排行榜接口，
// 支持分页、并列排名、升降序等业务功能.
// 基本用法:
//
//	// 内存实现
//	lb := leaderboard.NewMemoryLeaderboard("daily_score")
//	// Redis 实现
//	lb := leaderboard.NewRedisLeaderboard(redisClient, "daily_score")
//	lb.AddScore(ctx, "player1", 100)
//	lb.IncrScore(ctx, "player1", 50)
//	top, _ := lb.TopN(ctx, 10)
package leaderboard

import (
	"context"
	"sort"
	"sync"

	"github.com/redis/go-redis/v9"
)

// Entry 排行榜条目.
type Entry struct {
	Member string  `json:"member"`
	Score  float64 `json:"score"`
	Rank   int64   `json:"rank"` // 1-based
}

// Page 分页结果.
type Page struct {
	Entries []Entry `json:"entries"`
	Total   int64   `json:"total"`
	HasMore bool    `json:"has_more"`
}

// Order 排序方式.
type Order int

const (
	// Descending 降序（默认，分数高的排前面）.
	Descending Order = iota
	// Ascending 升序（分数低的排前面）.
	Ascending
)

// Leaderboard 排行榜接口.
type Leaderboard interface {
	// AddScore 设置成员分数（覆盖）.
	AddScore(ctx context.Context, member string, score float64) error
	// IncrScore 增加成员分数，返回新分数.
	IncrScore(ctx context.Context, member string, delta float64) (float64, error)
	// GetRank 获取成员排名.
	GetRank(ctx context.Context, member string) (*Entry, error)
	// GetScore 获取成员分数.
	GetScore(ctx context.Context, member string) (float64, error)
	// TopN 获取前 N 名.
	TopN(ctx context.Context, n int) ([]Entry, error)
	// GetPage 分页获取排行榜.
	GetPage(ctx context.Context, offset, limit int) (*Page, error)
	// Remove 移除成员.
	Remove(ctx context.Context, members ...string) error
	// Count 获取排行榜成员总数.
	Count(ctx context.Context) (int64, error)
	// Reset 重置排行榜.
	Reset(ctx context.Context) error
}

// options 排行榜选项.
type options struct {
	prefix string
	order  Order
}

// Option 排行榜选项函数.
type Option func(*options)

// WithPrefix 设置键前缀.
func WithPrefix(prefix string) Option {
	return func(o *options) {
		o.prefix = prefix
	}
}

// WithOrder 设置排序方式.
func WithOrder(order Order) Option {
	return func(o *options) {
		o.order = order
	}
}

func applyOptions(opts []Option) *options {
	o := &options{order: Descending}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// ---- 内存实现 ----

type memEntry struct {
	member string
	score  float64
}

// memoryLeaderboard 基于内存的排行榜实现.
type memoryLeaderboard struct {
	mu      sync.RWMutex
	name    string
	members map[string]float64
	opts    *options
}

// NewMemoryLeaderboard 创建内存排行榜.
func NewMemoryLeaderboard(name string, opts ...Option) Leaderboard {
	return &memoryLeaderboard{
		name:    name,
		members: make(map[string]float64),
		opts:    applyOptions(opts),
	}
}

func (lb *memoryLeaderboard) sorted() []memEntry {
	entries := make([]memEntry, 0, len(lb.members))
	for m, s := range lb.members {
		entries = append(entries, memEntry{member: m, score: s})
	}
	if lb.opts.order == Descending {
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].score != entries[j].score {
				return entries[i].score > entries[j].score
			}
			return entries[i].member < entries[j].member
		})
	} else {
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].score != entries[j].score {
				return entries[i].score < entries[j].score
			}
			return entries[i].member < entries[j].member
		})
	}
	return entries
}

func (lb *memoryLeaderboard) AddScore(_ context.Context, member string, score float64) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.members[member] = score
	return nil
}

func (lb *memoryLeaderboard) IncrScore(_ context.Context, member string, delta float64) (float64, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.members[member] += delta
	return lb.members[member], nil
}

func (lb *memoryLeaderboard) GetRank(_ context.Context, member string) (*Entry, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	score, ok := lb.members[member]
	if !ok {
		return nil, nil
	}
	sorted := lb.sorted()
	for i, e := range sorted {
		if e.member == member {
			return &Entry{Member: member, Score: score, Rank: int64(i + 1)}, nil
		}
	}
	return nil, nil
}

func (lb *memoryLeaderboard) GetScore(_ context.Context, member string) (float64, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return lb.members[member], nil
}

func (lb *memoryLeaderboard) TopN(_ context.Context, n int) ([]Entry, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	sorted := lb.sorted()
	if n > len(sorted) {
		n = len(sorted)
	}
	result := make([]Entry, n)
	for i := 0; i < n; i++ {
		result[i] = Entry{Member: sorted[i].member, Score: sorted[i].score, Rank: int64(i + 1)}
	}
	return result, nil
}

func (lb *memoryLeaderboard) GetPage(_ context.Context, offset, limit int) (*Page, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	sorted := lb.sorted()
	total := int64(len(sorted))

	if offset >= len(sorted) {
		return &Page{Entries: []Entry{}, Total: total, HasMore: false}, nil
	}

	end := offset + limit
	if end > len(sorted) {
		end = len(sorted)
	}

	entries := make([]Entry, 0, end-offset)
	for i := offset; i < end; i++ {
		entries = append(entries, Entry{
			Member: sorted[i].member,
			Score:  sorted[i].score,
			Rank:   int64(i + 1),
		})
	}

	return &Page{
		Entries: entries,
		Total:   total,
		HasMore: end < len(sorted),
	}, nil
}

func (lb *memoryLeaderboard) Remove(_ context.Context, members ...string) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	for _, m := range members {
		delete(lb.members, m)
	}
	return nil
}

func (lb *memoryLeaderboard) Count(_ context.Context) (int64, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return int64(len(lb.members)), nil
}

func (lb *memoryLeaderboard) Reset(_ context.Context) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.members = make(map[string]float64)
	return nil
}

// ---- Redis 实现 ----

// redisLeaderboard 基于 Redis Sorted Set 的排行榜实现.
type redisLeaderboard struct {
	client redis.Cmdable
	name   string
	opts   *options
}

// NewRedisLeaderboard 创建 Redis 排行榜.
func NewRedisLeaderboard(client redis.Cmdable, name string, opts ...Option) Leaderboard {
	return &redisLeaderboard{
		client: client,
		name:   name,
		opts:   applyOptions(opts),
	}
}

func (lb *redisLeaderboard) key() string {
	return lb.opts.prefix + "lb:" + lb.name
}

func (lb *redisLeaderboard) AddScore(ctx context.Context, member string, score float64) error {
	return lb.client.ZAdd(ctx, lb.key(), redis.Z{Score: score, Member: member}).Err()
}

func (lb *redisLeaderboard) IncrScore(ctx context.Context, member string, delta float64) (float64, error) {
	return lb.client.ZIncrBy(ctx, lb.key(), delta, member).Result()
}

func (lb *redisLeaderboard) GetRank(ctx context.Context, member string) (*Entry, error) {
	var rankCmd *redis.IntCmd
	var scoreCmd *redis.FloatCmd
	pipe := lb.client.Pipeline()
	if lb.opts.order == Descending {
		rankCmd = pipe.ZRevRank(ctx, lb.key(), member)
	} else {
		rankCmd = pipe.ZRank(ctx, lb.key(), member)
	}
	scoreCmd = pipe.ZScore(ctx, lb.key(), member)
	if _, err := pipe.Exec(ctx); err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	return &Entry{
		Member: member,
		Score:  scoreCmd.Val(),
		Rank:   rankCmd.Val() + 1,
	}, nil
}

func (lb *redisLeaderboard) GetScore(ctx context.Context, member string) (float64, error) {
	score, err := lb.client.ZScore(ctx, lb.key(), member).Result()
	if err == redis.Nil {
		return 0, nil
	}
	return score, err
}

func (lb *redisLeaderboard) TopN(ctx context.Context, n int) ([]Entry, error) {
	return lb.getRange(ctx, 0, int64(n-1))
}

func (lb *redisLeaderboard) getRange(ctx context.Context, start, stop int64) ([]Entry, error) {
	var zs []redis.Z
	var err error
	if lb.opts.order == Descending {
		zs, err = lb.client.ZRevRangeWithScores(ctx, lb.key(), start, stop).Result()
	} else {
		zs, err = lb.client.ZRangeWithScores(ctx, lb.key(), start, stop).Result()
	}
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, len(zs))
	for i, z := range zs {
		entries[i] = Entry{
			Member: z.Member.(string),
			Score:  z.Score,
			Rank:   start + int64(i) + 1,
		}
	}
	return entries, nil
}

func (lb *redisLeaderboard) GetPage(ctx context.Context, offset, limit int) (*Page, error) {
	total, err := lb.client.ZCard(ctx, lb.key()).Result()
	if err != nil {
		return nil, err
	}
	entries, err := lb.getRange(ctx, int64(offset), int64(offset+limit-1))
	if err != nil {
		return nil, err
	}
	return &Page{
		Entries: entries,
		Total:   total,
		HasMore: int64(offset+limit) < total,
	}, nil
}

func (lb *redisLeaderboard) Remove(ctx context.Context, members ...string) error {
	ifaces := make([]any, len(members))
	for i, m := range members {
		ifaces[i] = m
	}
	return lb.client.ZRem(ctx, lb.key(), ifaces...).Err()
}

func (lb *redisLeaderboard) Count(ctx context.Context) (int64, error) {
	return lb.client.ZCard(ctx, lb.key()).Result()
}

func (lb *redisLeaderboard) Reset(ctx context.Context) error {
	return lb.client.Del(ctx, lb.key()).Err()
}
