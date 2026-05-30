// Package abtesting 提供 A/B 测试功能，支持流量分桶、多变体实验和结果追踪.
package abtesting

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"sync"
	"time"
)

var (
	// ErrExperimentNotFound 实验不存在.
	ErrExperimentNotFound = errors.New("abtesting: experiment not found")
	// ErrExperimentDisabled 实验已禁用.
	ErrExperimentDisabled = errors.New("abtesting: experiment disabled")
	// ErrNoVariants 实验没有变体.
	ErrNoVariants = errors.New("abtesting: no variants")
	// ErrInvalidWeight 无效的权重配置.
	ErrInvalidWeight = errors.New("abtesting: invalid weight")
)

// Variant 实验变体.
type Variant struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Weight   int            `json:"weight"`
	Metadata map[string]any `json:"metadata,omitzero"`
}

// Experiment A/B 实验.
type Experiment struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	Variants  []Variant `json:"variants"`
	Salt      string    `json:"salt"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// Assignment 用户分组分配.
type Assignment struct {
	ExperimentID string    `json:"experiment_id"`
	VariantID    string    `json:"variant_id"`
	UserID       string    `json:"user_id"`
	AssignedAt   time.Time `json:"assigned_at"`
}

// ExposureEvent 曝光事件.
type ExposureEvent struct {
	ExperimentID string    `json:"experiment_id"`
	VariantID    string    `json:"variant_id"`
	UserID       string    `json:"user_id"`
	Timestamp    time.Time `json:"timestamp"`
}

// Store A/B 测试持久化接口.
type Store interface {
	// SaveExperiment 保存实验.
	SaveExperiment(ctx context.Context, exp *Experiment) error
	// GetExperiment 获取实验.
	GetExperiment(ctx context.Context, id string) (*Experiment, error)
	// ListExperiments 列出所有实验.
	ListExperiments(ctx context.Context) ([]*Experiment, error)
	// UpdateExperiment 更新实验.
	UpdateExperiment(ctx context.Context, exp *Experiment) error
	// SaveAssignment 保存分配记录.
	SaveAssignment(ctx context.Context, assignment *Assignment) error
	// GetAssignment 获取分配记录.
	GetAssignment(ctx context.Context, experimentID, userID string) (*Assignment, error)
	// SaveExposure 保存曝光事件.
	SaveExposure(ctx context.Context, event *ExposureEvent) error
}

// Option 管理器配置选项.
type Option func(*Manager)

// WithLogger 设置日志记录器.
func WithLogger(logger *slog.Logger) Option {
	return func(m *Manager) {
		m.logger = logger
	}
}

// Manager A/B 测试管理器.
type Manager struct {
	store  Store
	logger *slog.Logger
}

// New 创建 A/B 测试管理器.
func New(store Store, opts ...Option) *Manager {
	m := &Manager{
		store:  store,
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// CreateExperiment 创建实验.
func (m *Manager) CreateExperiment(ctx context.Context, exp *Experiment) error {
	if len(exp.Variants) == 0 {
		return ErrNoVariants
	}
	totalWeight := 0
	for _, v := range exp.Variants {
		if v.Weight < 0 {
			return fmt.Errorf("%w: variant %q has negative weight %d", ErrInvalidWeight, v.ID, v.Weight)
		}
		totalWeight += v.Weight
	}
	if totalWeight != 100 {
		return fmt.Errorf("%w: total weight must be 100, got %d", ErrInvalidWeight, totalWeight)
	}
	return m.store.SaveExperiment(ctx, exp)
}

// GetExperiment 获取实验.
func (m *Manager) GetExperiment(ctx context.Context, id string) (*Experiment, error) {
	return m.store.GetExperiment(ctx, id)
}

// Assign 分配用户到实验变体基于确定性哈希.
func (m *Manager) Assign(ctx context.Context, experimentID, userID string) (*Assignment, error) {
	// 先检查是否已有分配
	existing, err := m.store.GetAssignment(ctx, experimentID, userID)
	if err == nil && existing != nil {
		return existing, nil
	}

	exp, err := m.store.GetExperiment(ctx, experimentID)
	if err != nil {
		return nil, err
	}
	if !exp.Enabled {
		return nil, ErrExperimentDisabled
	}
	if len(exp.Variants) == 0 {
		return nil, ErrNoVariants
	}

	// 使用 FNV 哈希确定性分桶
	variantID := assignVariant(exp, userID)

	assignment := &Assignment{
		ExperimentID: experimentID,
		VariantID:    variantID,
		UserID:       userID,
		AssignedAt:   time.Now(),
	}
	if err := m.store.SaveAssignment(ctx, assignment); err != nil {
		return nil, err
	}
	return assignment, nil
}

// GetAssignment 获取已有的分配记录.
func (m *Manager) GetAssignment(ctx context.Context, experimentID, userID string) (*Assignment, error) {
	return m.store.GetAssignment(ctx, experimentID, userID)
}

// TrackExposure 记录曝光事件.
func (m *Manager) TrackExposure(ctx context.Context, event *ExposureEvent) error {
	return m.store.SaveExposure(ctx, event)
}

// ListExperiments 列出所有实验.
func (m *Manager) ListExperiments(ctx context.Context) ([]*Experiment, error) {
	return m.store.ListExperiments(ctx)
}

// DisableExperiment 禁用实验.
func (m *Manager) DisableExperiment(ctx context.Context, id string) error {
	exp, err := m.store.GetExperiment(ctx, id)
	if err != nil {
		return err
	}
	exp.Enabled = false
	return m.store.UpdateExperiment(ctx, exp)
}

// assignVariant 根据确定性哈希分配变体.
func assignVariant(exp *Experiment, userID string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(exp.Salt + ":" + exp.ID + ":" + userID))
	bucket := int(h.Sum32() % 100)

	cumulative := 0
	for _, v := range exp.Variants {
		cumulative += v.Weight
		if bucket < cumulative {
			return v.ID
		}
	}
	// 兜底返回最后一个变体
	return exp.Variants[len(exp.Variants)-1].ID
}

// --- Memory Store ---

// MemoryStore 基于内存的 A/B 测试存储用于测试.
type MemoryStore struct {
	mu          sync.RWMutex
	experiments map[string]*Experiment
	assignments map[string]*Assignment // key: experimentID + ":" + userID
	exposures   []*ExposureEvent
}

// NewMemoryStore 创建基于内存的 A/B 测试存储.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		experiments: make(map[string]*Experiment),
		assignments: make(map[string]*Assignment),
	}
}

func assignmentKey(experimentID, userID string) string {
	return experimentID + ":" + userID
}

// SaveExperiment 保存实验.
func (s *MemoryStore) SaveExperiment(_ context.Context, exp *Experiment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *exp
	cp.Variants = make([]Variant, len(exp.Variants))
	copy(cp.Variants, exp.Variants)
	s.experiments[exp.ID] = &cp
	return nil
}

// GetExperiment 获取实验.
func (s *MemoryStore) GetExperiment(_ context.Context, id string) (*Experiment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	exp, ok := s.experiments[id]
	if !ok {
		return nil, ErrExperimentNotFound
	}
	cp := *exp
	cp.Variants = make([]Variant, len(exp.Variants))
	copy(cp.Variants, exp.Variants)
	return &cp, nil
}

// ListExperiments 列出所有实验.
func (s *MemoryStore) ListExperiments(_ context.Context) ([]*Experiment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Experiment, 0, len(s.experiments))
	for _, exp := range s.experiments {
		cp := *exp
		cp.Variants = make([]Variant, len(exp.Variants))
		copy(cp.Variants, exp.Variants)
		result = append(result, &cp)
	}
	return result, nil
}

// UpdateExperiment 更新实验.
func (s *MemoryStore) UpdateExperiment(_ context.Context, exp *Experiment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.experiments[exp.ID]; !ok {
		return ErrExperimentNotFound
	}
	cp := *exp
	cp.Variants = make([]Variant, len(exp.Variants))
	copy(cp.Variants, exp.Variants)
	s.experiments[exp.ID] = &cp
	return nil
}

// SaveAssignment 保存分配记录.
func (s *MemoryStore) SaveAssignment(_ context.Context, assignment *Assignment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *assignment
	s.assignments[assignmentKey(assignment.ExperimentID, assignment.UserID)] = &cp
	return nil
}

// GetAssignment 获取分配记录.
func (s *MemoryStore) GetAssignment(_ context.Context, experimentID, userID string) (*Assignment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.assignments[assignmentKey(experimentID, userID)]
	if !ok {
		return nil, ErrExperimentNotFound
	}
	cp := *a
	return &cp, nil
}

// SaveExposure 保存曝光事件.
func (s *MemoryStore) SaveExposure(_ context.Context, event *ExposureEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *event
	s.exposures = append(s.exposures, &cp)
	return nil
}
