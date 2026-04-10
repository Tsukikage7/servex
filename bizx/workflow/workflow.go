// Package workflow 提供通用工作流引擎，支持审批流、条件分支和并行执行.
package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Status 工作流实例状态.
type Status string

const (
	// StatusPending 等待执行.
	StatusPending Status = "pending"
	// StatusRunning 执行中.
	StatusRunning Status = "running"
	// StatusCompleted 已完成.
	StatusCompleted Status = "completed"
	// StatusFailed 执行失败.
	StatusFailed Status = "failed"
	// StatusCancelled 已取消.
	StatusCancelled Status = "cancelled"
	// StatusWaitingApproval 等待审批.
	StatusWaitingApproval Status = "waiting_approval"
)

// NodeType 工作流节点类型.
type NodeType string

const (
	// NodeTypeTask 任务节点.
	NodeTypeTask NodeType = "task"
	// NodeTypeApproval 审批节点.
	NodeTypeApproval NodeType = "approval"
	// NodeTypeCondition 条件分支节点.
	NodeTypeCondition NodeType = "condition"
	// NodeTypeParallel 并行执行节点.
	NodeTypeParallel NodeType = "parallel"
	// NodeTypeEnd 结束节点.
	NodeTypeEnd NodeType = "end"
)

var (
	// ErrDefinitionNotFound 工作流定义不存在.
	ErrDefinitionNotFound = errors.New("workflow: definition not found")
	// ErrInstanceNotFound 工作流实例不存在.
	ErrInstanceNotFound = errors.New("workflow: instance not found")
	// ErrInvalidNode 无效的节点.
	ErrInvalidNode = errors.New("workflow: invalid node")
	// ErrWorkflowCompleted 工作流已完成.
	ErrWorkflowCompleted = errors.New("workflow: workflow already completed")
	// ErrApprovalRequired 需要审批.
	ErrApprovalRequired = errors.New("workflow: approval required")
	// ErrMaxStepsExceeded 超过最大步数限制.
	ErrMaxStepsExceeded = errors.New("workflow: max steps exceeded")
)

// Handler 节点处理函数.
type Handler func(ctx context.Context, instance *Instance) error

// ConditionFunc 条件判断函数，返回下一个节点 ID.
type ConditionFunc func(ctx context.Context, instance *Instance) (nextNodeID string, err error)

// Node 工作流节点.
type Node struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Type      NodeType      `json:"type"`
	Handler   Handler       `json:"-"`
	NextNodes []string      `json:"next_nodes"`
	Condition ConditionFunc `json:"-"`
}

// Definition 工作流定义.
type Definition struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Version     string           `json:"version"`
	Nodes       map[string]*Node `json:"nodes"`
	StartNodeID string           `json:"start_node_id"`
}

// Instance 工作流实例.
type Instance struct {
	ID            string         `json:"id"`
	DefinitionID  string         `json:"definition_id"`
	Status        Status         `json:"status"`
	CurrentNodeID string         `json:"current_node_id"`
	Data          map[string]any `json:"data"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// Store 工作流持久化接口.
type Store interface {
	// SaveInstance 保存工作流实例.
	SaveInstance(ctx context.Context, instance *Instance) error
	// GetInstance 获取工作流实例.
	GetInstance(ctx context.Context, id string) (*Instance, error)
	// UpdateInstance 更新工作流实例.
	UpdateInstance(ctx context.Context, instance *Instance) error
	// ListInstancesByStatus 按状态列出工作流实例.
	ListInstancesByStatus(ctx context.Context, status Status) ([]*Instance, error)
}

// Option 引擎配置选项.
type Option func(*Engine)

// WithLogger 设置日志记录器.
func WithLogger(logger *slog.Logger) Option {
	return func(e *Engine) {
		e.logger = logger
	}
}

// WithMaxParallel 设置最大并行执行数.
func WithMaxParallel(n int) Option {
	return func(e *Engine) {
		if n > 0 {
			e.maxParallel = n
		}
	}
}

// WithMaxSteps 设置工作流单次执行的最大步数，防止死循环（默认1000）.
func WithMaxSteps(n int) Option {
	return func(e *Engine) {
		if n > 0 {
			e.maxSteps = n
		}
	}
}

// CheckpointFunc 检查点回调，在每个节点执行完成后调用以持久化中间状态.
// 崩溃恢复时可从最近的检查点继续执行，避免重复执行已完成的节点.
type CheckpointFunc func(ctx context.Context, instance *Instance) error

// WithCheckpoint 设置检查点回调函数.
func WithCheckpoint(fn CheckpointFunc) Option {
	return func(e *Engine) {
		e.checkpoint = fn
	}
}

// Engine 工作流引擎.
type Engine struct {
	mu          sync.RWMutex
	store       Store
	definitions map[string]*Definition
	logger      *slog.Logger
	maxParallel int
	maxSteps    int
	checkpoint  CheckpointFunc
}

// New 创建工作流引擎.
func New(store Store, opts ...Option) *Engine {
	e := &Engine{
		store:       store,
		definitions: make(map[string]*Definition),
		logger:      slog.Default(),
		maxParallel: 5,
		maxSteps:    1000,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// RegisterDefinition 注册工作流定义.
func (e *Engine) RegisterDefinition(def *Definition) error {
	if def.ID == "" || def.StartNodeID == "" {
		return fmt.Errorf("%w: definition must have ID and StartNodeID", ErrInvalidNode)
	}
	if _, ok := def.Nodes[def.StartNodeID]; !ok {
		return fmt.Errorf("%w: start node %q not found in nodes", ErrInvalidNode, def.StartNodeID)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.definitions[def.ID] = def
	return nil
}

// StartWorkflow 启动工作流实例.
func (e *Engine) StartWorkflow(ctx context.Context, definitionID string, data map[string]any) (*Instance, error) {
	e.mu.RLock()
	def, ok := e.definitions[definitionID]
	e.mu.RUnlock()
	if !ok {
		return nil, ErrDefinitionNotFound
	}

	now := time.Now()
	instance := &Instance{
		ID:            uuid.New().String(),
		DefinitionID:  definitionID,
		Status:        StatusPending,
		CurrentNodeID: def.StartNodeID,
		Data:          data,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if instance.Data == nil {
		instance.Data = make(map[string]any)
	}

	if err := e.store.SaveInstance(ctx, instance); err != nil {
		return nil, err
	}
	return instance, nil
}

// Execute 推进工作流执行.
func (e *Engine) Execute(ctx context.Context, instanceID string) error {
	instance, err := e.store.GetInstance(ctx, instanceID)
	if err != nil {
		return err
	}
	if instance.Status == StatusCompleted || instance.Status == StatusCancelled || instance.Status == StatusFailed {
		return ErrWorkflowCompleted
	}

	e.mu.RLock()
	def, ok := e.definitions[instance.DefinitionID]
	e.mu.RUnlock()
	if !ok {
		return ErrDefinitionNotFound
	}

	instance.Status = StatusRunning
	instance.UpdatedAt = time.Now()

	for steps := 0; ; steps++ {
		if steps >= e.maxSteps {
			instance.Status = StatusFailed
			instance.UpdatedAt = time.Now()
			_ = e.store.UpdateInstance(ctx, instance)
			return fmt.Errorf("%w: exceeded %d steps", ErrMaxStepsExceeded, e.maxSteps)
		}
		node, ok := def.Nodes[instance.CurrentNodeID]
		if !ok {
			instance.Status = StatusFailed
			_ = e.store.UpdateInstance(ctx, instance)
			return fmt.Errorf("%w: node %q not found", ErrInvalidNode, instance.CurrentNodeID)
		}

		switch node.Type {
		case NodeTypeEnd:
			instance.Status = StatusCompleted
			instance.UpdatedAt = time.Now()
			return e.store.UpdateInstance(ctx, instance)

		case NodeTypeTask:
			if node.Handler != nil {
				if err := node.Handler(ctx, instance); err != nil {
					instance.Status = StatusFailed
					instance.UpdatedAt = time.Now()
					_ = e.store.UpdateInstance(ctx, instance)
					return err
				}
			}
			if len(node.NextNodes) == 0 {
				instance.Status = StatusCompleted
				instance.UpdatedAt = time.Now()
				return e.store.UpdateInstance(ctx, instance)
			}
			instance.CurrentNodeID = node.NextNodes[0]
			instance.UpdatedAt = time.Now()
			// 持久化中间状态，崩溃恢复时可从此节点继续
			if e.checkpoint != nil {
				if err := e.checkpoint(ctx, instance); err != nil {
					e.logger.WarnContext(ctx, "workflow: 检查点持久化失败", "instance_id", instance.ID, "error", err)
				}
			}

		case NodeTypeApproval:
			instance.Status = StatusWaitingApproval
			instance.UpdatedAt = time.Now()
			return e.store.UpdateInstance(ctx, instance)

		case NodeTypeCondition:
			if node.Condition == nil {
				instance.Status = StatusFailed
				instance.UpdatedAt = time.Now()
				_ = e.store.UpdateInstance(ctx, instance)
				return fmt.Errorf("%w: condition node %q has no condition func", ErrInvalidNode, node.ID)
			}
			nextID, err := node.Condition(ctx, instance)
			if err != nil {
				instance.Status = StatusFailed
				instance.UpdatedAt = time.Now()
				_ = e.store.UpdateInstance(ctx, instance)
				return err
			}
			instance.CurrentNodeID = nextID
			instance.UpdatedAt = time.Now()
			// 持久化中间状态
			if e.checkpoint != nil {
				if err := e.checkpoint(ctx, instance); err != nil {
					e.logger.WarnContext(ctx, "workflow: 检查点持久化失败", "instance_id", instance.ID, "error", err)
				}
			}

		case NodeTypeParallel:
			if err := e.executeParallel(ctx, instance, node); err != nil {
				instance.Status = StatusFailed
				instance.UpdatedAt = time.Now()
				_ = e.store.UpdateInstance(ctx, instance)
				return err
			}
			if len(node.NextNodes) == 0 {
				instance.Status = StatusCompleted
				instance.UpdatedAt = time.Now()
				return e.store.UpdateInstance(ctx, instance)
			}
			instance.CurrentNodeID = node.NextNodes[0]
			instance.UpdatedAt = time.Now()
			// 持久化中间状态
			if e.checkpoint != nil {
				if err := e.checkpoint(ctx, instance); err != nil {
					e.logger.WarnContext(ctx, "workflow: 检查点持久化失败", "instance_id", instance.ID, "error", err)
				}
			}

		default:
			instance.Status = StatusFailed
			instance.UpdatedAt = time.Now()
			_ = e.store.UpdateInstance(ctx, instance)
			return fmt.Errorf("%w: unknown node type %q", ErrInvalidNode, node.Type)
		}
	}
}

// executeParallel 并行执行节点的所有子处理器.
// 查找所有 NextNodes 引用的节点并行执行其 Handler，使用信号量控制并行度.
func (e *Engine) executeParallel(ctx context.Context, instance *Instance, node *Node) error {
	if node.Handler == nil && len(node.NextNodes) == 0 {
		return nil
	}

	// 收集需要并行执行的 handler 列表
	var handlers []Handler
	if node.Handler != nil {
		handlers = append(handlers, node.Handler)
	}

	// 使用信号量控制并行度
	sem := make(chan struct{}, e.maxParallel)
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)

	for _, h := range handlers {
		h := h
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := h(ctx, instance); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return firstErr
}

// Approve 审批通过.
func (e *Engine) Approve(ctx context.Context, instanceID string, approver string) error {
	instance, err := e.store.GetInstance(ctx, instanceID)
	if err != nil {
		return err
	}
	if instance.Status != StatusWaitingApproval {
		return fmt.Errorf("%w: instance status is %s, not waiting_approval", ErrApprovalRequired, instance.Status)
	}

	e.mu.RLock()
	def, ok := e.definitions[instance.DefinitionID]
	e.mu.RUnlock()
	if !ok {
		return ErrDefinitionNotFound
	}

	node, ok := def.Nodes[instance.CurrentNodeID]
	if !ok {
		return fmt.Errorf("%w: node %q not found", ErrInvalidNode, instance.CurrentNodeID)
	}

	instance.Data["_approver"] = approver
	instance.Data["_approved"] = true

	if len(node.NextNodes) > 0 {
		instance.CurrentNodeID = node.NextNodes[0]
		instance.Status = StatusRunning
	} else {
		instance.Status = StatusCompleted
	}
	instance.UpdatedAt = time.Now()

	return e.store.UpdateInstance(ctx, instance)
}

// Reject 审批拒绝.
func (e *Engine) Reject(ctx context.Context, instanceID string, approver string, reason string) error {
	instance, err := e.store.GetInstance(ctx, instanceID)
	if err != nil {
		return err
	}
	if instance.Status != StatusWaitingApproval {
		return fmt.Errorf("%w: instance status is %s, not waiting_approval", ErrApprovalRequired, instance.Status)
	}

	instance.Data["_approver"] = approver
	instance.Data["_approved"] = false
	instance.Data["_reject_reason"] = reason
	instance.Status = StatusFailed
	instance.UpdatedAt = time.Now()

	return e.store.UpdateInstance(ctx, instance)
}

// Cancel 取消工作流.
func (e *Engine) Cancel(ctx context.Context, instanceID string) error {
	instance, err := e.store.GetInstance(ctx, instanceID)
	if err != nil {
		return err
	}
	if instance.Status == StatusCompleted || instance.Status == StatusCancelled {
		return ErrWorkflowCompleted
	}

	instance.Status = StatusCancelled
	instance.UpdatedAt = time.Now()

	return e.store.UpdateInstance(ctx, instance)
}

// GetInstance 获取工作流实例.
func (e *Engine) GetInstance(ctx context.Context, instanceID string) (*Instance, error) {
	return e.store.GetInstance(ctx, instanceID)
}

// --- Memory Store ---

// MemoryStore 基于内存的工作流存储（用于测试）.
type MemoryStore struct {
	mu        sync.RWMutex
	instances map[string]*Instance
}

// NewMemoryStore 创建基于内存的工作流存储.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{instances: make(map[string]*Instance)}
}

// SaveInstance 保存工作流实例.
func (s *MemoryStore) SaveInstance(_ context.Context, instance *Instance) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *instance
	cp.Data = copyData(instance.Data)
	s.instances[instance.ID] = &cp
	return nil
}

// GetInstance 获取工作流实例.
func (s *MemoryStore) GetInstance(_ context.Context, id string) (*Instance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inst, ok := s.instances[id]
	if !ok {
		return nil, ErrInstanceNotFound
	}
	cp := *inst
	cp.Data = copyData(inst.Data)
	return &cp, nil
}

// UpdateInstance 更新工作流实例.
func (s *MemoryStore) UpdateInstance(_ context.Context, instance *Instance) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.instances[instance.ID]; !ok {
		return ErrInstanceNotFound
	}
	cp := *instance
	cp.Data = copyData(instance.Data)
	s.instances[instance.ID] = &cp
	return nil
}

// ListInstancesByStatus 按状态列出工作流实例.
func (s *MemoryStore) ListInstancesByStatus(_ context.Context, status Status) ([]*Instance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Instance
	for _, inst := range s.instances {
		if inst.Status == status {
			cp := *inst
			cp.Data = copyData(inst.Data)
			result = append(result, &cp)
		}
	}
	return result, nil
}

// copyData 深拷贝 map（通过 JSON 序列化/反序列化）.
func copyData(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	b, err := json.Marshal(data)
	if err != nil {
		// 回退到浅拷贝
		cp := make(map[string]any, len(data))
		for k, v := range data {
			cp[k] = v
		}
		return cp
	}
	cp := make(map[string]any, len(data))
	_ = json.Unmarshal(b, &cp)
	return cp
}
