// Package saga 提供 Saga 分布式事务编排.
// Saga 模式用于管理跨服务的分布式事务，通过编排一系列本地事务，
// 并在失败时执行补偿操作来保证最终一致性。
// 基本用法:
//
//	saga := saga.New("create-order").
//	    Step("reserve-inventory", reserveInventory, compensateInventory).
//	    Step("charge-payment", chargePayment, refundPayment).
//	    Step("send-notification", sendNotification, nil).
//	    Build()
//	if err := saga.Execute(ctx); err != nil {
//	    // 失败时会自动执行补偿
//	    log.Error("saga failed", err)
//	}
//
// 数据传递:
//
//	reserveInventory := func(ctx context.Context, data *saga.Data) error {
//	    orderID := data.GetString("order_id")
//	    // 执行业务逻辑
//	    data.Set("reservation_id", "RES-123")
//	    return nil
//	}
//	chargePayment := func(ctx context.Context, data *saga.Data) error {
//	    reservationID := data.GetString("reservation_id")
//	    // 使用上一步的数据
//	    return nil
//	}
package saga

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Saga 表示一个 Saga 事务.
type Saga struct {
	name  string
	steps []Step
	opts  *options
	idGen func() string
}

// Builder Saga 构建器.
type Builder struct {
	name  string
	steps []Step
	opts  []Option
}

// New 创建 Saga 构建器.
func New(name string) *Builder {
	return &Builder{
		name:  name,
		steps: make([]Step, 0),
	}
}

// Step 添加步骤.
// name: 步骤名称
// action: 正向操作
// compensate: 补偿操作可选，传 nil 表示不需要补偿
func (b *Builder) Step(name string, action StepFunc, compensate CompensateFunc) *Builder {
	b.steps = append(b.steps, Step{
		Name:       name,
		Action:     action,
		Compensate: compensate,
	})
	return b
}

// Options 设置配置选项.
func (b *Builder) Options(opts ...Option) *Builder {
	b.opts = append(b.opts, opts...)
	return b
}

// Build 构建 Saga.
func (b *Builder) Build() (*Saga, error) {
	if len(b.steps) == 0 {
		return nil, fmt.Errorf("saga: 没有定义步骤")
	}

	return &Saga{
		name:  b.name,
		steps: b.steps,
		opts:  applyOptions(b.opts),
		idGen: defaultIDGenerator,
	}, nil
}

// defaultIDGenerator 默认 ID 生成器UUID 避免碰撞.
func defaultIDGenerator() string {
	return uuid.New().String()
}

// Execute 执行 Saga.
// 执行所有步骤，如果任何步骤失败，会按逆序执行已完成步骤的补偿操作.
func (s *Saga) Execute(ctx context.Context) error {
	return s.ExecuteWithData(ctx, NewData())
}

// ExecuteWithData 使用指定的共享数据执行 Saga.
func (s *Saga) ExecuteWithData(ctx context.Context, data *Data) error {
	// 创建状态每次执行独立的状态实例，无需全局互斥
	state := NewState(s.idGen(), s.name, len(s.steps))
	for i, step := range s.steps {
		state.StepResults[i].StepName = step.Name
	}

	// 保存初始状态
	state.Status = SagaStatusRunning
	if err := s.opts.store.Save(ctx, state); err != nil {
		if s.opts.logger != nil {
			logger.ForOr(ctx, s.opts.logger, "Saga").Warn(
				"保存初始状态失败",
				logger.String("saga", s.name),
				logger.Err(err),
			)
		}
	}

	// 设置超时
	if s.opts.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.opts.timeout)
		defer cancel()
	}

	// 执行步骤
	var lastErr error
	completedSteps := 0

	for i, step := range s.steps {
		// 检查 context
		if ctx.Err() != nil {
			lastErr = ctx.Err()
			break
		}

		// 更新状态
		state.CurrentStep = i
		state.StepResults[i].Status = StepStatusRunning

		// 执行钩子
		if s.opts.onStepStart != nil {
			s.opts.onStepStart(step.Name)
		}

		// 记录开始时间
		startTime := time.Now()

		// 执行步骤带重试
		err := s.executeStepWithRetry(ctx, step, data)

		// 记录执行时间
		state.StepResults[i].Duration = time.Since(startTime).Milliseconds()

		// 执行钩子
		if s.opts.onStepEnd != nil {
			s.opts.onStepEnd(step.Name, err)
		}

		if err != nil {
			state.StepResults[i].Status = StepStatusFailed
			state.StepResults[i].Error = err
			lastErr = err

			if s.opts.logger != nil {
				logger.ForOr(ctx, s.opts.logger, "Saga").Error(
					"步骤执行失败",
					logger.String("saga", s.name),
					logger.String("step", step.Name),
					logger.Err(err),
				)
			}

			break
		}

		state.StepResults[i].Status = StepStatusCompleted
		completedSteps = i + 1

		if s.opts.logger != nil {
			logger.ForOr(ctx, s.opts.logger, "Saga").Debug(
				"步骤执行完成",
				logger.String("saga", s.name),
				logger.String("step", step.Name),
				logger.Int64("duration_ms", state.StepResults[i].Duration),
			)
		}
	}

	// 所有步骤成功
	if lastErr == nil {
		state.Status = SagaStatusCompleted
		now := time.Now()
		state.CompletedAt = &now
		_ = s.saveState(ctx, state)

		if s.opts.logger != nil {
			logger.ForOr(ctx, s.opts.logger, "Saga").Info(
				"执行成功",
				logger.String("saga", s.name),
			)
		}

		return nil
	}

	// 执行补偿
	state.Status = SagaStatusCompensating
	state.Error = lastErr.Error()
	_ = s.saveState(ctx, state)

	if s.opts.logger != nil {
		logger.ForOr(ctx, s.opts.logger, "Saga").Info(
			"开始执行补偿",
			logger.String("saga", s.name),
			logger.Int("completed_steps", completedSteps),
		)
	}

	compensateErr := s.compensate(ctx, data, state, completedSteps)

	now := time.Now()
	state.CompletedAt = &now

	if compensateErr != nil {
		state.Status = SagaStatusCompensateFailed
		if saveErr := s.saveState(ctx, state); saveErr != nil {
			return fmt.Errorf("%w: %v (compensation failed: %v, state save failed: %v)", ErrSagaFailed, lastErr, compensateErr, saveErr)
		}
		return fmt.Errorf("%w: %v (compensation failed: %v)", ErrSagaFailed, lastErr, compensateErr)
	}

	state.Status = SagaStatusCompensated
	if saveErr := s.saveState(ctx, state); saveErr != nil {
		return fmt.Errorf("%w: %v (state save failed: %v)", ErrSagaFailed, lastErr, saveErr)
	}
	return fmt.Errorf("%w: %v", ErrSagaFailed, lastErr)
}

// executeStepWithRetry 带重试执行步骤.
func (s *Saga) executeStepWithRetry(ctx context.Context, step Step, data *Data) error {
	var lastErr error

	for attempt := 0; attempt <= s.opts.retryCount; attempt++ {
		if attempt > 0 {
			// 等待重试
			retryTimer := time.NewTimer(s.opts.retryDelay)
			select {
			case <-ctx.Done():
				retryTimer.Stop()
				return ctx.Err()
			case <-retryTimer.C:
			}

			if s.opts.logger != nil {
				logger.ForOr(ctx, s.opts.logger, "Saga").Debug(
					"重试步骤",
					logger.String("step", step.Name),
					logger.Int("attempt", attempt+1),
				)
			}
		}

		err := step.Action(ctx, data)
		if err == nil {
			return nil
		}

		lastErr = err
	}

	return lastErr
}

// compensate 执行补偿操作.
func (s *Saga) compensate(ctx context.Context, data *Data, state *State, completedSteps int) error {
	var lastErr error

	// 逆序执行补偿
	for i := completedSteps - 1; i >= 0; i-- {
		step := s.steps[i]

		// 没有补偿函数，跳过
		if step.Compensate == nil {
			state.StepResults[i].Status = StepStatusCompensated
			continue
		}

		state.StepResults[i].Status = StepStatusCompensating

		err := step.Compensate(ctx, data)
		if err != nil {
			state.StepResults[i].Status = StepStatusCompensateFailed
			state.StepResults[i].Error = err
			lastErr = err

			if s.opts.logger != nil {
				logger.ForOr(ctx, s.opts.logger, "Saga").Error(
					"补偿执行失败",
					logger.String("saga", s.name),
					logger.String("step", step.Name),
					logger.Err(err),
				)
			}

			// 继续执行其他补偿
			continue
		}

		state.StepResults[i].Status = StepStatusCompensated

		if s.opts.logger != nil {
			logger.ForOr(ctx, s.opts.logger, "Saga").Debug(
				"步骤已补偿",
				logger.String("saga", s.name),
				logger.String("step", step.Name),
			)
		}
	}

	return lastErr
}

// saveState 保存状态. 返回保存错误.
func (s *Saga) saveState(ctx context.Context, state *State) error {
	if err := s.opts.store.Save(ctx, state); err != nil {
		if s.opts.logger != nil {
			logger.ForOr(ctx, s.opts.logger, "Saga").Warn(
				"保存状态失败",
				logger.String("saga", s.name),
				logger.String("status", string(state.Status)),
				logger.Err(err),
			)
		}
		return err
	}
	return nil
}

// Name 返回 Saga 名称.
func (s *Saga) Name() string {
	return s.name
}

// Steps 返回步骤数量.
func (s *Saga) Steps() int {
	return len(s.steps)
}
